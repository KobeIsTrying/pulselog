package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pulselog/pulselog/internal/httpx"
)

type readiness struct {
	kafka func(context.Context) error
	ch    func(context.Context) error
}

func newHandler(log *slog.Logger, ready readiness) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ready.kafka(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not_ready", "message": "kafka is unreachable"})
			return
		}
		if err := ready.ch(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not_ready", "message": "clickhouse is unreachable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", promhttp.Handler())
	return httpx.Chain(mux,
		httpx.Recover(log),
		httpx.RequestID,
		httpx.Metrics,
		httpx.AccessLog(log),
	)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
