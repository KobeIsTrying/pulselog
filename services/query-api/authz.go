package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/auth"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/metrics"
)

func (s *Server) principal(r *http.Request) (*identity.Principal, error) {
	if s.authn != nil {
		return s.authn(r)
	}
	tok := auth.BearerToken(r)
	if tok == "" {
		return nil, auth.ErrUnauthorized
	}
	if s.tokens == nil {
		return nil, auth.ErrUnauthorized
	}
	claims, err := s.tokens.Parse(tok)
	if err != nil {
		return nil, auth.ErrUnauthorized
	}
	if s.deny != nil {
		denied, err := s.deny.Denied(r.Context(), claims.ID)
		if err != nil {
			return nil, err
		}
		if denied {
			return nil, auth.ErrUnauthorized
		}
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, auth.ErrUnauthorized
	}
	if s.dir == nil {
		return nil, auth.ErrUnauthorized
	}
	p, err := s.dir.LoadPrincipal(r.Context(), uid)
	if err != nil {
		return nil, auth.ErrUnauthorized
	}
	p.JTI = claims.ID
	if claims.ExpiresAt != nil {
		p.TokenExp = claims.ExpiresAt.Time
	}
	return p, nil
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (*identity.Principal, bool) {
	if s.limit != nil && !s.limit.Allow(r.Context(), "query", auth.ClientIP(r), s.rate.QueryLimit, s.rate.QueryWindow) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited", "message": "too many requests"})
		return nil, false
	}
	p, err := s.principal(r)
	if err != nil {
		metrics.AuthFailure.WithLabelValues("jwt", "invalid").Inc()
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "authentication required"})
		return nil, false
	}
	return p, true
}

func (s *Server) requirePerm(w http.ResponseWriter, r *http.Request, orgID uuid.UUID, perm auth.Permission) (*identity.Principal, bool) {
	p, ok := s.requireUser(w, r)
	if !ok {
		return nil, false
	}
	if !p.Can(orgID, perm) {
		metrics.AuthzDenied.WithLabelValues("role").Inc()
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "insufficient permissions"})
		return nil, false
	}
	return p, true
}

func (s *Server) authorizeProjects(w http.ResponseWriter, r *http.Request, requested []uuid.UUID) ([]uuid.UUID, bool) {
	p, ok := s.requireUser(w, r)
	if !ok {
		return nil, false
	}
	if len(requested) == 1 {
		if !p.HasProject(requested[0]) {
			metrics.AuthzDenied.WithLabelValues("project").Inc()
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "project is not accessible"})
			return nil, false
		}
		return requested, true
	}
	return p.ProjectIDs, true
}

func remainingTTL(exp time.Time) time.Duration {
	d := time.Until(exp)
	if d < time.Minute {
		return time.Minute
	}
	return d
}

func auditSafe(_ context.Context, err error) {
	_ = err
}

var errDirUnavailable = errors.New("identity store unavailable")
