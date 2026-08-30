package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/pulselog/pulselog/internal/auth"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/models"
)

type ingestResponse struct {
	Accepted int      `json:"accepted"`
	EventIDs []string `json:"event_ids"`
}

type errorResponse struct {
	Error   string              `json:"error"`
	Message string              `json:"message,omitempty"`
	Fields  []models.FieldError `json:"fields,omitempty"`
}

func (s *Server) handleIngestOne(w http.ResponseWriter, r *http.Request) {
	key, ok := s.authenticateKey(w, r)
	if !ok {
		return
	}
	var ev models.LogEvent
	if err := s.decodeJSON(w, r, &ev); err != nil {
		return
	}
	ev.Normalize(time.Now())
	if err := bindTenant(&ev, key); err != nil {
		s.writeSpoof(w)
		return
	}
	if err := ev.Validate(); err != nil {
		s.writeValidation(w, err)
		return
	}
	if err := s.publish(r.Context(), []models.LogEvent{ev}); err != nil {
		s.writePublishError(w, err)
		return
	}
	metrics.EventsAccepted.Inc()
	writeJSON(w, http.StatusAccepted, ingestResponse{Accepted: 1, EventIDs: []string{ev.EventID}})
}

func (s *Server) handleIngestBatch(w http.ResponseWriter, r *http.Request) {
	key, ok := s.authenticateKey(w, r)
	if !ok {
		return
	}
	var req models.BatchRequest
	if err := s.decodeJSON(w, r, &req); err != nil {
		return
	}
	if err := req.ValidateSize(); err != nil {
		s.writeValidation(w, err)
		return
	}
	now := time.Now()
	ids := make([]string, 0, len(req.Events))
	for i := range req.Events {
		req.Events[i].Normalize(now)
		if err := bindTenant(&req.Events[i], key); err != nil {
			s.writeSpoof(w)
			return
		}
		ids = append(ids, req.Events[i].EventID)
	}
	if err := models.ValidateBatch(req.Events); err != nil {
		s.writeValidation(w, err)
		return
	}
	if err := s.publish(r.Context(), req.Events); err != nil {
		s.writePublishError(w, err)
		return
	}
	metrics.EventsAccepted.Add(float64(len(req.Events)))
	writeJSON(w, http.StatusAccepted, ingestResponse{Accepted: len(req.Events), EventIDs: ids})
}

func (s *Server) publish(parent context.Context, events []models.LogEvent) error {
	ctx, cancel := context.WithTimeout(parent, s.cfg.Kafka.WriteTimeout)
	defer cancel()
	return s.pub.Publish(ctx, events)
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil {
		metrics.EventsRejected.WithLabelValues("invalid_json").Inc()
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is required")
			return err
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds limit")
			return err
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return err
	}
	return nil
}

func (s *Server) writeValidation(w http.ResponseWriter, err error) {
	metrics.EventsRejected.WithLabelValues("validation").Inc()
	var ve *models.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:  "validation_failed",
			Fields: ve.Fields,
		})
		return
	}
	writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
}

func (s *Server) writePublishError(w http.ResponseWriter, err error) {
	s.log.Error("kafka publish failed", "err", err)
	writeError(w, http.StatusServiceUnavailable, "kafka_unavailable", "failed to publish events")
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authenticateKey(w http.ResponseWriter, r *http.Request) (identity.IngestKey, bool) {
	raw := auth.APIKeyFromRequest(r)
	ip := auth.ClientIP(r)
	if s.limit != nil && raw == "" && !s.limit.Allow(r.Context(), "ingest", "ip:"+ip, s.cfg.RateLimit.IngestLimit, s.cfg.RateLimit.IngestWindow) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return identity.IngestKey{}, false
	}
	if raw == "" {
		metrics.APIKeyRejected.WithLabelValues("missing").Inc()
		writeError(w, http.StatusUnauthorized, "unauthorized", "api key required")
		return identity.IngestKey{}, false
	}
	if s.keys == nil {
		metrics.APIKeyRejected.WithLabelValues("unavailable").Inc()
		writeError(w, http.StatusServiceUnavailable, "unavailable", "api key verifier unavailable")
		return identity.IngestKey{}, false
	}
	key, err := s.keys.Verify(r.Context(), raw)
	if err != nil {
		reason := "invalid"
		if errors.Is(err, identity.ErrRevoked) {
			reason = "revoked"
		}
		metrics.APIKeyRejected.WithLabelValues(reason).Inc()
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid api key")
		return identity.IngestKey{}, false
	}
	if s.limit != nil && !s.limit.Allow(r.Context(), "ingest", "key:"+key.ID.String(), s.cfg.RateLimit.IngestLimit, s.cfg.RateLimit.IngestWindow) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return identity.IngestKey{}, false
	}
	metrics.AuthSuccess.WithLabelValues("api_key").Inc()
	return key, true
}

func bindTenant(ev *models.LogEvent, key identity.IngestKey) error {
	if ev.Service != "" && ev.Service != key.Service {
		return errSpoof
	}
	if ev.ProjectID != "" && ev.ProjectID != key.ProjectID.String() {
		return errSpoof
	}
	ev.Service = key.Service
	ev.ProjectID = key.ProjectID.String()
	return nil
}

var errSpoof = errors.New("tenant spoofing")

func (s *Server) writeSpoof(w http.ResponseWriter) {
	metrics.AuthzDenied.WithLabelValues("ingest_spoof").Inc()
	writeError(w, http.StatusForbidden, "forbidden", "service or project does not match api key")
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pub.Ready(ctx); err != nil {
		s.log.Warn("readiness check failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "not_ready", "kafka is unreachable")
		return
	}
	for _, check := range s.readies {
		if err := check(ctx); err != nil {
			s.log.Warn("readiness check failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "not_ready", "dependency is unreachable")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
