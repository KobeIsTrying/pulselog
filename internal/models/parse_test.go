package models

import (
	"errors"
	"testing"
	"time"
)

func TestParseJSONOK(t *testing.T) {
	raw := []byte(`{"service":"payment-service","level":"ERROR","message":"denied","event_id":"11111111-1111-1111-1111-111111111111","timestamp":"2026-08-29T11:00:00Z"}`)
	ev, err := ParseJSON(raw, time.Now().UTC())
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if ev.Service != "payment-service" || ev.EventID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestParseJSONEmptyAndMalformed(t *testing.T) {
	if _, err := ParseJSON(nil, time.Now()); err == nil {
		t.Fatal("empty should fail")
	}
	if _, err := ParseJSON([]byte("{"), time.Now()); err == nil {
		t.Fatal("malformed should fail")
	}
}

func TestParseJSONValidation(t *testing.T) {
	_, err := ParseJSON([]byte(`{"service":"api","level":"TRACE","message":"x"}`), time.Now())
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}
