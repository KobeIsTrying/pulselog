package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/httpx"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/models"
	"github.com/pulselog/pulselog/internal/ratelimit"
)

type Publisher interface {
	Publish(ctx context.Context, events []models.LogEvent) error
	Ready(ctx context.Context) error
}

type KeyVerifier interface {
	Verify(ctx context.Context, raw string) (identity.IngestKey, error)
}

type Server struct {
	cfg     config.Config
	log     *slog.Logger
	pub     Publisher
	keys    KeyVerifier
	limit   ratelimit.Gate
	readies []func(context.Context) error
}

func NewServer(cfg config.Config, log *slog.Logger, pub Publisher, keys KeyVerifier, limit ratelimit.Gate, readies ...func(context.Context) error) *Server {
	return &Server{cfg: cfg, log: log, pub: pub, keys: keys, limit: limit, readies: readies}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/logs", s.handleIngestOne)
	mux.HandleFunc("POST /v1/logs/batch", s.handleIngestBatch)
	mux.HandleFunc("POST /ingest", s.handleIngestOne)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.Handle("GET /metrics", promhttp.Handler())

	return httpx.Chain(mux,
		httpx.Recover(s.log),
		httpx.RequestID,
		httpx.SecurityHeaders,
		httpx.CORS(s.cfg.Auth.CORSOrigins),
		httpx.Metrics,
		httpx.AccessLog(s.log),
	)
}
