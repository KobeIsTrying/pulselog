package models

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeAndValidateOK(t *testing.T) {
	ev := LogEvent{
		Service: "payment-service",
		Level:   "error",
		Message: "Payment authorization failed",
		Metadata: map[string]string{
			"requestId": "req-1",
			"traceId":   "tr-9",
		},
	}
	ev.Normalize(time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC))
	if err := ev.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ev.Level != "ERROR" {
		t.Fatalf("Level = %q", ev.Level)
	}
	if ev.EventID == "" {
		t.Fatal("expected event id")
	}
	if ev.TraceID != "tr-9" {
		t.Fatalf("TraceID = %q", ev.TraceID)
	}
	if !ev.Timestamp.Equal(time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("Timestamp = %s", ev.Timestamp)
	}
}

func TestNormalizePreservesExistingEventID(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	ev := LogEvent{Service: "api", Level: "INFO", Message: "ok", EventID: id}
	ev.Normalize(time.Now())
	if ev.EventID != id {
		t.Fatalf("EventID changed: %q", ev.EventID)
	}
}

func TestValidateRejectsBadServiceAndLevel(t *testing.T) {
	ev := LogEvent{Service: "bad service", Level: "TRACE", Message: "x"}
	ev.Normalize(time.Now())
	err := ev.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("type %T", err)
	}
	if len(ve.Fields) < 2 {
		t.Fatalf("fields = %+v", ve.Fields)
	}
}

func TestValidateMessageLength(t *testing.T) {
	ev := LogEvent{Service: "api", Level: "INFO", Message: strings.Repeat("a", MaxMessageLen+1)}
	ev.Normalize(time.Now())
	if err := ev.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateBatchIndexes(t *testing.T) {
	events := []LogEvent{
		{Service: "api", Level: "INFO", Message: "ok"},
		{Service: "", Level: "INFO", Message: "missing"},
	}
	now := time.Now()
	for i := range events {
		events[i].Normalize(now)
	}
	err := ValidateBatch(events)
	if err == nil {
		t.Fatal("expected error")
	}
	ve := err.(*ValidationError)
	if ve.Fields[0].Field != "events[1].service" {
		t.Fatalf("field = %q", ve.Fields[0].Field)
	}
}

func TestBatchRequestSize(t *testing.T) {
	if err := (BatchRequest{}).ValidateSize(); err == nil {
		t.Fatal("empty should fail")
	}
	tooBig := BatchRequest{Events: make([]LogEvent, MaxBatchEvents+1)}
	if err := tooBig.ValidateSize(); err == nil {
		t.Fatal("oversized should fail")
	}
}
