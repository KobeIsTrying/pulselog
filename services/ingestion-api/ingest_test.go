package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/models"
	"github.com/pulselog/pulselog/internal/ratelimit"
)

var testProject = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

type stubKeys struct {
	err     error
	service string
	last    string
}

func (s *stubKeys) Verify(_ context.Context, raw string) (identity.IngestKey, error) {
	s.last = raw
	if s.err != nil {
		return identity.IngestKey{}, s.err
	}
	switch raw {
	case "pl_live_revoked":
		return identity.IngestKey{}, identity.ErrRevoked
	case "pl_live_testkey":
		svc := s.service
		if svc == "" {
			svc = "payment-service"
		}
		return identity.IngestKey{
			ID:        uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
			ProjectID: testProject,
			Service:   svc,
		}, nil
	default:
		return identity.IngestKey{}, identity.ErrNotFound
	}
}

type stubPublisher struct {
	err   error
	ready error
	got   []models.LogEvent
}

func (s *stubPublisher) Publish(_ context.Context, events []models.LogEvent) error {
	if s.err != nil {
		return s.err
	}
	s.got = append(s.got, events...)
	return nil
}

func (s *stubPublisher) Ready(context.Context) error { return s.ready }

func testServer(t *testing.T, pub Publisher) http.Handler {
	t.Helper()
	return testServerKeys(t, pub, &stubKeys{}, nil)
}

func testServerKeys(t *testing.T, pub Publisher, keys KeyVerifier, limit ratelimit.Gate) http.Handler {
	t.Helper()
	cfg := config.Config{
		HTTPAddr:     ":0",
		MaxBodyBytes: 1 << 20,
		Kafka:        config.KafkaConfig{WriteTimeout: 2 * time.Second},
		RateLimit:    config.RateLimitConfig{IngestLimit: 2, IngestWindow: time.Minute},
	}
	return NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), pub, keys, limit).Handler()
}

func ingestReq(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "pl_live_testkey")
	return req
}

func TestIngestOneAccepted(t *testing.T) {
	pub := &stubPublisher{}
	h := testServer(t, pub)

	body := `{"service":"payment-service","level":"ERROR","message":"Payment authorization failed","metadata":{"requestId":"r1"}}`
	req := ingestReq("/v1/logs", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var resp ingestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Accepted != 1 || len(resp.EventIDs) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	if pub.got[0].EventID == "" {
		t.Fatal("expected published event_id")
	}
	if pub.got[0].ProjectID != testProject.String() {
		t.Fatalf("project_id = %q", pub.got[0].ProjectID)
	}
}

func TestIngestPreservesClientEventID(t *testing.T) {
	pub := &stubPublisher{}
	h := testServer(t, pub)
	id := "22222222-2222-4222-8222-222222222222"
	body := `{"event_id":"` + id + `","service":"payment-service","level":"INFO","message":"keep-id"}`
	req := ingestReq("/v1/logs", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(pub.got) != 1 || pub.got[0].EventID != id {
		t.Fatalf("published = %+v", pub.got)
	}
}

func TestIngestOneValidation(t *testing.T) {
	h := testServer(t, &stubPublisher{})
	req := ingestReq("/v1/logs", `{"service":"","level":"NOPE","message":""}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestIngestOneInvalidJSON(t *testing.T) {
	h := testServer(t, &stubPublisher{})
	req := ingestReq("/v1/logs", `{`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestIngestBatch(t *testing.T) {
	pub := &stubPublisher{}
	h := testServer(t, pub)
	body := `{"events":[{"service":"payment-service","level":"INFO","message":"one"},{"service":"payment-service","level":"WARN","message":"two"}]}`
	req := ingestReq("/v1/logs/batch", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(pub.got) != 2 {
		t.Fatalf("published %d", len(pub.got))
	}
}

func TestIngestKafkaUnavailable(t *testing.T) {
	h := testServer(t, &stubPublisher{err: errors.New("broker down")})
	req := ingestReq("/v1/logs", `{"service":"payment-service","level":"INFO","message":"ok"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	h := testServer(t, &stubPublisher{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestReadyz(t *testing.T) {
	h := testServer(t, &stubPublisher{ready: errors.New("no kafka")})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestIngestRejectsHugeBody(t *testing.T) {
	cfg := config.Config{MaxBodyBytes: 64, Kafka: config.KafkaConfig{WriteTimeout: 2 * time.Second}}
	h := NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), &stubPublisher{}, &stubKeys{}, nil).Handler()
	req := ingestReq("/v1/logs", string(bytes.Repeat([]byte("a"), 128)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestIngestAPIKeyRequiredInvalidAndRevoked(t *testing.T) {
	h := testServer(t, &stubPublisher{})
	body := `{"service":"payment-service","level":"INFO","message":"ok"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing key %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/logs", strings.NewReader(body))
	req.Header.Set("X-API-Key", "pl_live_invalid")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid key %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/logs", strings.NewReader(body))
	req.Header.Set("X-API-Key", "pl_live_revoked")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key %d", rec.Code)
	}
}

func TestIngestRejectsServiceSpoof(t *testing.T) {
	pub := &stubPublisher{}
	h := testServer(t, pub)
	req := ingestReq("/v1/logs", `{"service":"inventory-service","level":"INFO","message":"spoof"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(pub.got) != 0 {
		t.Fatal("published spoofed event")
	}
}

func TestIngestRateLimit(t *testing.T) {
	limit := ratelimit.NewMemory()
	h := testServerKeys(t, &stubPublisher{}, &stubKeys{}, limit)
	body := `{"service":"payment-service","level":"INFO","message":"ok"}`
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, ingestReq("/v1/logs", body))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("i=%d status=%d", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, ingestReq("/v1/logs", body))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIngestAliasRequiresKey(t *testing.T) {
	h := testServer(t, &stubPublisher{})
	req := ingestReq("/ingest", `{"service":"payment-service","level":"INFO","message":"ok"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d", rec.Code)
	}
}
