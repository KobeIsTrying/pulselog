package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pulselog/pulselog/internal/auth"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/httpx"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/ratelimit"
	"github.com/pulselog/pulselog/internal/realtime"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	log      *slog.Logger
	store    Store
	dir      *identity.Store
	tokens   *auth.Issuer
	deny     auth.DenyStore
	limit    ratelimit.Gate
	timeout  time.Duration
	signups  bool
	cors     []string
	rate     config.RateLimitConfig
	authn    func(*http.Request) (*identity.Principal, error)
	now      func() time.Time
	readies  []func(context.Context) error
	hub      *realtime.Hub
	rdb      *redis.Client
	tickets  realtime.TicketStore
	maxConns int
	wsBuffer int
	env      string
}

func NewServer(log *slog.Logger, store Store, timeout time.Duration) *Server {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Server{log: log, store: store, timeout: timeout, now: time.Now}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	mux.HandleFunc("GET /api/v1/orgs", s.handleListOrgs)
	mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects", s.handleCreateProject)
	mux.HandleFunc("GET /api/v1/orgs/{orgID}/members", s.handleListMembers)
	mux.HandleFunc("POST /api/v1/orgs/{orgID}/members", s.handleAddMember)
	mux.HandleFunc("PATCH /api/v1/orgs/{orgID}/members/{userID}", s.handleUpdateMember)
	mux.HandleFunc("DELETE /api/v1/orgs/{orgID}/members/{userID}", s.handleRemoveMember)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/services", s.handleListServices)
	mux.HandleFunc("POST /api/v1/projects/{projectID}/services", s.handleCreateService)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/api-keys", s.handleListKeys)
	mux.HandleFunc("POST /api/v1/projects/{projectID}/api-keys", s.handleCreateKey)
	mux.HandleFunc("DELETE /api/v1/api-keys/{keyID}", s.handleRevokeKey)
	mux.HandleFunc("GET /api/v1/logs", s.handleListLogs)
	mux.HandleFunc("GET /api/v1/logs/{eventID}", s.handleGetLog)
	mux.HandleFunc("GET /api/v1/stats/overview", s.handleOverview)
	mux.HandleFunc("GET /api/v1/stats/timeseries", s.handleTimeseries)
	mux.HandleFunc("GET /api/v1/stats/services", s.handleServices)
	mux.HandleFunc("GET /api/v1/stats/errors", s.handleErrors)
	mux.HandleFunc("POST /api/v1/stream/ticket", s.handleStreamTicket)
	mux.HandleFunc("GET /api/v1/stream", s.handleStream)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.Handle("GET /metrics", promhttp.Handler())
	return httpx.Chain(mux,
		httpx.Recover(s.log),
		httpx.RequestID,
		httpx.SecurityHeaders,
		httpx.CORS(s.cors),
		httpx.Metrics,
		httpx.AccessLog(s.log),
	)
}

type listResponse struct {
	Logs       []LogRow `json:"logs"`
	PageSize   int      `json:"page_size"`
	HasMore    bool     `json:"has_more"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

func (s *Server) withTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), s.timeout)
}

func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r.URL.Query())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ids, ok := s.authorizeProjects(w, r, q.ProjectIDs)
	if !ok {
		return
	}
	q.ProjectIDs = ids
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	rows, cursor, more, qerr := s.store.ListLogs(ctx, q)
	if qerr != nil {
		s.writeStoreError(w, qerr)
		return
	}
	if rows == nil {
		rows = []LogRow{}
	}
	writeJSON(w, http.StatusOK, listResponse{
		Logs:       rows,
		PageSize:   q.Limit,
		HasMore:    more,
		NextCursor: cursor,
	})
}

func (s *Server) handleGetLog(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("eventID"))
	if err != nil {
		writeAPIError(w, badRequest("eventID", "must be a UUID"))
		return
	}
	ids, ok := s.authorizeProjects(w, r, nil)
	if !ok {
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	row, qerr := s.store.GetLog(ctx, id, ids)
	if qerr != nil {
		s.writeStoreError(w, qerr)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	tr, err := parseTimeRange(r.URL.Query(), true)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	requested, perr := parseRequestedProjects(r.URL.Query())
	if perr != nil {
		writeAPIError(w, perr)
		return
	}
	ids, ok := s.authorizeProjects(w, r, requested)
	if !ok {
		return
	}
	tr.ProjectIDs = ids
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	out, qerr := s.store.Overview(ctx, tr)
	if qerr != nil {
		s.writeStoreError(w, qerr)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleTimeseries(w http.ResponseWriter, r *http.Request) {
	q, err := parseTimeseriesQuery(r.URL.Query())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	requested, perr := parseRequestedProjects(r.URL.Query())
	if perr != nil {
		writeAPIError(w, perr)
		return
	}
	ids, ok := s.authorizeProjects(w, r, requested)
	if !ok {
		return
	}
	q.ProjectIDs = ids
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	out, qerr := s.store.Timeseries(ctx, q)
	if qerr != nil {
		s.writeStoreError(w, qerr)
		return
	}
	if out == nil {
		out = []TimeBucket{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"interval": q.Interval, "points": out})
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	q, err := parseServiceStatsQuery(r.URL.Query())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	requested, perr := parseRequestedProjects(r.URL.Query())
	if perr != nil {
		writeAPIError(w, perr)
		return
	}
	ids, ok := s.authorizeProjects(w, r, requested)
	if !ok {
		return
	}
	q.ProjectIDs = ids
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	out, qerr := s.store.Services(ctx, q)
	if qerr != nil {
		s.writeStoreError(w, qerr)
		return
	}
	if out == nil {
		out = []ServiceStat{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": out})
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	q, err := parseErrorStatsQuery(r.URL.Query())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	requested, perr := parseRequestedProjects(r.URL.Query())
	if perr != nil {
		writeAPIError(w, perr)
		return
	}
	ids, ok := s.authorizeProjects(w, r, requested)
	if !ok {
		return
	}
	q.ProjectIDs = ids
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	out, qerr := s.store.TopErrors(ctx, q)
	if qerr != nil {
		s.writeStoreError(w, qerr)
		return
	}
	if out == nil {
		out = []ErrorGroup{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"errors": out})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		s.log.Warn("readiness check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "not_ready",
			"message": "clickhouse is unreachable",
		})
		return
	}
	for _, check := range s.readies {
		if err := check(ctx); err != nil {
			s.log.Warn("readiness check failed", "err", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "not_ready",
				"message": "dependency is unreachable",
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "event not found"})
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		s.log.Warn("query timeout", "err", err)
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{"error": "timeout", "message": "query timed out"})
		return
	}
	s.log.Error("query failed", "err", err)
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error":   "unavailable",
		"message": "query backend unavailable",
	})
}

func writeAPIError(w http.ResponseWriter, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, map[string]any{
			"error":   ae.Code,
			"message": ae.Msg,
			"fields":  ae.Fields,
		})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_query", "message": "invalid request"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
