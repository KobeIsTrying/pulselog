package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	h := newHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), readiness{
		kafka: func(context.Context) error { return nil },
		ch:    func(context.Context) error { return nil },
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestReadyzClickHouseDown(t *testing.T) {
	h := newHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), readiness{
		kafka: func(context.Context) error { return nil },
		ch:    func(context.Context) error { return context.DeadlineExceeded },
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["message"] != "clickhouse is unreachable" {
		t.Fatalf("body = %v", body)
	}
}
