package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/auth"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/metrics"
)

type registerReq struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	Organization string `json:"organization"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.signups {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "signups are disabled"})
		return
	}
	if !s.limitLogin(w, r) {
		return
	}
	var req registerReq
	if !s.decodeAuth(w, r, &req) {
		return
	}
	if s.dir == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable", "message": "identity store unavailable"})
		return
	}
	user, org, project, err := s.dir.Register(r.Context(), req.Email, req.Password, req.Organization)
	if err != nil {
		if errors.Is(err, identity.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "conflict", "message": "email already registered"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": err.Error()})
		return
	}
	if s.tokens == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal", "message": "token issuer unavailable"})
		return
	}
	tok, _, exp, err := s.tokens.Issue(user.ID, user.Email, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal", "message": "could not issue token"})
		return
	}
	metrics.AuthSuccess.WithLabelValues("jwt").Inc()
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":        tok,
		"token_type":   "Bearer",
		"expires_at":   exp.UTC().Format(time.RFC3339),
		"user_id":      user.ID,
		"email":        user.Email,
		"organization": org,
		"project":      project,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.limitLogin(w, r) {
		return
	}
	var req loginReq
	if !s.decodeAuth(w, r, &req) {
		return
	}
	if s.dir == nil || s.tokens == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable", "message": "identity store unavailable"})
		return
	}
	user, err := s.dir.AuthenticateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		metrics.AuthFailure.WithLabelValues("password", "invalid").Inc()
		if s.dir != nil {
			_ = s.dir.Audit(r.Context(), uuid.Nil, nil, "login.failed", "user", identity.NormalizeEmail(req.Email), nil)
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "invalid credentials"})
		return
	}
	metrics.AuthSuccess.WithLabelValues("password").Inc()
	_ = s.dir.Audit(r.Context(), uuid.Nil, &user.ID, "login.success", "user", user.ID.String(), nil)
	s.issueToken(w, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if s.deny != nil && p.JTI != "" {
		_ = s.deny.Deny(r.Context(), p.JTI, remainingTTL(p.TokenExp))
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	orgs := []identity.OrgMembership{}
	if s.dir != nil {
		if got, err := s.dir.ListOrgs(r.Context(), p.UserID); err == nil && got != nil {
			orgs = got
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":     p.UserID,
		"email":       p.Email,
		"orgs":        orgs,
		"project_ids": p.ProjectIDs,
	})
}

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if s.dir == nil {
		writeJSON(w, http.StatusOK, map[string]any{"orgs": []any{}})
		return
	}
	orgs, err := s.dir.ListOrgs(r.Context(), p.UserID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orgs": orgs})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseID(w, r, "orgID")
	if !ok {
		return
	}
	if _, ok := s.requirePerm(w, r, orgID, auth.PermLogsRead); !ok {
		return
	}
	projects, err := s.dir.ListProjects(r.Context(), orgID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseID(w, r, "orgID")
	if !ok {
		return
	}
	if _, ok := s.requirePerm(w, r, orgID, auth.PermProjectsManage); !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !s.decodeAuth(w, r, &req) {
		return
	}
	p, err := s.dir.CreateProject(r.Context(), orgID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseID(w, r, "orgID")
	if !ok {
		return
	}
	if _, ok := s.requirePerm(w, r, orgID, auth.PermLogsRead); !ok {
		return
	}
	members, err := s.dir.ListMembers(r.Context(), orgID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseID(w, r, "orgID")
	if !ok {
		return
	}
	actor, ok := s.requirePerm(w, r, orgID, auth.PermMembersManage)
	if !ok {
		return
	}
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !s.decodeAuth(w, r, &req) {
		return
	}
	role, okRole := auth.ParseRole(req.Role)
	if !okRole {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "invalid role"})
		return
	}
	user, err := s.dir.UserByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "user not found"})
		return
	}
	if err := s.dir.AddMember(r.Context(), orgID, actor.UserID, user.ID, role); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "conflict", "message": "member already exists"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseID(w, r, "orgID")
	if !ok {
		return
	}
	userID, ok := parseID(w, r, "userID")
	if !ok {
		return
	}
	actor, ok := s.requirePerm(w, r, orgID, auth.PermMembersManage)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !s.decodeAuth(w, r, &req) {
		return
	}
	role, okRole := auth.ParseRole(req.Role)
	if !okRole {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "invalid role"})
		return
	}
	if err := s.dir.UpdateMemberRole(r.Context(), orgID, actor.UserID, userID, role); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "member not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseID(w, r, "orgID")
	if !ok {
		return
	}
	userID, ok := parseID(w, r, "userID")
	if !ok {
		return
	}
	actor, ok := s.requirePerm(w, r, orgID, auth.PermMembersManage)
	if !ok {
		return
	}
	if err := s.dir.RemoveMember(r.Context(), orgID, actor.UserID, userID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "member not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	project, actor, ok := s.projectPerm(w, r, auth.PermLogsRead)
	if !ok {
		return
	}
	_ = actor
	svcs, err := s.dir.ListServices(r.Context(), project.ID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": svcs})
}

func (s *Server) handleCreateService(w http.ResponseWriter, r *http.Request) {
	project, _, ok := s.projectPerm(w, r, auth.PermServicesManage)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !s.decodeAuth(w, r, &req) {
		return
	}
	svc, err := s.dir.CreateService(r.Context(), project.ID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	project, _, ok := s.projectPerm(w, r, auth.PermAPIKeysManage)
	if !ok {
		return
	}
	keys, err := s.dir.ListAPIKeys(r.Context(), project.ID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	project, actor, ok := s.projectPerm(w, r, auth.PermAPIKeysManage)
	if !ok {
		return
	}
	var req struct {
		Name    string `json:"name"`
		Service string `json:"service"`
	}
	if !s.decodeAuth(w, r, &req) {
		return
	}
	svc, err := s.dir.ServiceByName(r.Context(), project.ID, strings.TrimSpace(req.Service))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "unknown service"})
		return
	}
	key, raw, err := s.dir.CreateAPIKey(r.Context(), project.ID, svc.ID, actor.UserID, req.Name)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"prefix":     key.Prefix,
		"name":       key.Name,
		"service":    svc.Name,
		"project_id": key.ProjectID,
		"token":      raw,
	})
}

func (s *Server) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	keyID, ok := parseID(w, r, "keyID")
	if !ok {
		return
	}
	p, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	projectID, err := s.dir.APIKeyProject(r.Context(), keyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "api key not found"})
		return
	}
	project, err := s.dir.Project(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "api key not found"})
		return
	}
	if !p.Can(project.OrgID, auth.PermAPIKeysManage) {
		metrics.AuthzDenied.WithLabelValues("role").Inc()
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "insufficient permissions"})
		return
	}
	if _, err := s.dir.RevokeAPIKey(r.Context(), keyID, p.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "api key not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) projectPerm(w http.ResponseWriter, r *http.Request, perm auth.Permission) (identity.Project, *identity.Principal, bool) {
	var zero identity.Project
	projectID, ok := parseID(w, r, "projectID")
	if !ok {
		return zero, nil, false
	}
	if s.dir == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable", "message": "identity store unavailable"})
		return zero, nil, false
	}
	project, err := s.dir.Project(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "project not found"})
		return zero, nil, false
	}
	actor, ok := s.requirePerm(w, r, project.OrgID, perm)
	if !ok {
		return zero, nil, false
	}
	return project, actor, true
}

func (s *Server) issueToken(w http.ResponseWriter, user identity.User) {
	if s.tokens == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal", "message": "token issuer unavailable"})
		return
	}
	tok, _, exp, err := s.tokens.Issue(user.ID, user.Email, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal", "message": "could not issue token"})
		return
	}
	metrics.AuthSuccess.WithLabelValues("jwt").Inc()
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      tok,
		"token_type": "Bearer",
		"expires_at": exp.UTC().Format(time.RFC3339),
		"user_id":    user.ID,
		"email":      user.Email,
	})
}

func (s *Server) limitLogin(w http.ResponseWriter, r *http.Request) bool {
	if s.limit != nil && !s.limit.Allow(r.Context(), "login", auth.ClientIP(r), s.rate.LoginLimit, s.rate.LoginWindow) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited", "message": "too many requests"})
		return false
	}
	return true
}

func (s *Server) decodeAuth(w http.ResponseWriter, r *http.Request, dest any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dest); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json", "message": "request body is not valid JSON"})
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": name + " must be a UUID"})
		return uuid.Nil, false
	}
	return id, true
}
