package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/pulselog/pulselog/internal/kafka"
	"github.com/pulselog/pulselog/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

type stubStore struct {
	failTimes int
	calls     int
	got       []models.LogEvent
	err       error
}

func (s *stubStore) Insert(_ context.Context, events []models.LogEvent) error {
	s.calls++
	if s.calls <= s.failTimes {
		if s.err != nil {
			return s.err
		}
		return errors.New("clickhouse unavailable")
	}
	s.got = append(s.got, events...)
	return nil
}

type stubDLQ struct {
	got []kafka.DeadLetter
	err error
}

func (d *stubDLQ) Publish(_ context.Context, letters []kafka.DeadLetter) error {
	if d.err != nil {
		return d.err
	}
	d.got = append(d.got, letters...)
	return nil
}

func testProcessor(store Store, dlq DLQ) *Processor {
	p := NewProcessor(slog.New(slog.NewTextHandler(io.Discard, nil)), store, dlq, "logs-ingest", 5, time.Millisecond)
	p.sleep = func(time.Duration) {}
	p.now = func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	return p
}

func validMsg(service, message string) kafkago.Message {
	body := `{"service":"` + service + `","level":"ERROR","message":"` + message + `","event_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}`
	if service == "other" {
		body = `{"service":"other-service","level":"INFO","message":"` + message + `","event_id":"bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee"}`
	}
	return kafkago.Message{Topic: "logs-ingest", Partition: 1, Offset: 7, Value: []byte(body)}
}

func TestProcessWritesValidEvents(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	err := p.Process(context.Background(), []kafkago.Message{
		validMsg("payment-service", "Payment authorization failed"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 1 || store.got[0].Service != "payment-service" {
		t.Fatalf("store = %+v", store.got)
	}
	if store.got[0].EventID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("event_id not preserved: %q", store.got[0].EventID)
	}
	if len(dlq.got) != 0 {
		t.Fatalf("dlq = %+v", dlq.got)
	}
}

func TestProcessInvalidJSONGoesToDLQ(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	err := p.Process(context.Background(), []kafkago.Message{
		{Topic: "logs-ingest", Partition: 0, Offset: 3, Value: []byte("{not-json")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 0 {
		t.Fatalf("store = %+v", store.got)
	}
	if len(dlq.got) != 1 || dlq.got[0].Reason != kafka.ReasonInvalidJSON {
		t.Fatalf("dlq = %+v", dlq.got)
	}
	if !strings.Contains(dlq.got[0].Raw, "not-json") {
		t.Fatalf("raw = %q", dlq.got[0].Raw)
	}
	if dlq.got[0].SourceOffset != 3 {
		t.Fatalf("offset = %d", dlq.got[0].SourceOffset)
	}
}

func TestProcessValidationGoesToDLQ(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	err := p.Process(context.Background(), []kafkago.Message{
		{Value: []byte(`{"service":"api","level":"TRACE","message":"nope"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 0 {
		t.Fatal("expected no insert")
	}
	if len(dlq.got) != 1 || dlq.got[0].Reason != kafka.ReasonValidation {
		t.Fatalf("dlq = %+v", dlq.got)
	}
}

func TestProcessRetriesThenSucceeds(t *testing.T) {
	store := &stubStore{failTimes: 2}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	err := p.Process(context.Background(), []kafkago.Message{validMsg("payment-service", "ok")})
	if err != nil {
		t.Fatal(err)
	}
	if store.calls != 3 {
		t.Fatalf("calls = %d", store.calls)
	}
	if len(store.got) != 1 {
		t.Fatal("expected write after retries")
	}
	if len(dlq.got) != 0 {
		t.Fatalf("dlq = %+v", dlq.got)
	}
}

func TestProcessClickHouseExhaustedGoesToDLQ(t *testing.T) {
	store := &stubStore{failTimes: 100}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	err := p.Process(context.Background(), []kafkago.Message{validMsg("payment-service", "ok")})
	if err != nil {
		t.Fatal(err)
	}
	if store.calls != 5 {
		t.Fatalf("calls = %d", store.calls)
	}
	if len(store.got) != 0 {
		t.Fatal("expected no successful write")
	}
	if len(dlq.got) != 1 || dlq.got[0].Reason != kafka.ReasonClickHouse {
		t.Fatalf("dlq = %+v", dlq.got)
	}
	if dlq.got[0].Attempts != 5 {
		t.Fatalf("attempts = %d", dlq.got[0].Attempts)
	}
}

func TestProcessDLQPublishFailureBlocksCommit(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{err: errors.New("broker down")}
	p := testProcessor(store, dlq)
	err := p.Process(context.Background(), []kafkago.Message{
		{Value: []byte("nope")},
	})
	if err == nil {
		t.Fatal("expected error so offsets are not committed")
	}
}

func TestProcessBatchSplitsValidAndInvalid(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	err := p.Process(context.Background(), []kafkago.Message{
		validMsg("payment-service", "ok"),
		{Value: []byte("{")},
		validMsg("other", "also-ok"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 2 {
		t.Fatalf("written = %d", len(store.got))
	}
	if len(dlq.got) != 1 {
		t.Fatalf("dlq = %d", len(dlq.got))
	}
}

func TestProcessDedupesEventIDInBatch(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	msg := validMsg("payment-service", "dup")
	err := p.Process(context.Background(), []kafkago.Message{msg, msg})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.got) != 1 {
		t.Fatalf("written = %d", len(store.got))
	}
}

type stubFanout struct {
	calls  int
	events []models.LogEvent
}

func (s *stubFanout) Publish(_ context.Context, events []models.LogEvent) error {
	s.calls++
	s.events = append(s.events, events...)
	return nil
}

func TestProcessPublishesAfterClickHouseWrite(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{}
	fan := &stubFanout{}
	p := testProcessor(store, dlq)
	p.fanout = fan
	err := p.Process(context.Background(), []kafkago.Message{
		validMsg("payment-service", "Payment authorization failed"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if fan.calls != 1 || len(fan.events) != 1 {
		t.Fatalf("fanout calls=%d events=%d", fan.calls, len(fan.events))
	}
}

func TestProcessSkipsFanoutWhenClickHouseFails(t *testing.T) {
	store := &stubStore{failTimes: 100}
	dlq := &stubDLQ{}
	fan := &stubFanout{}
	p := testProcessor(store, dlq)
	p.fanout = fan
	err := p.Process(context.Background(), []kafkago.Message{validMsg("payment-service", "ok")})
	if err != nil {
		t.Fatal(err)
	}
	if fan.calls != 0 {
		t.Fatalf("published before durable write: %d", fan.calls)
	}
}

func TestBackoffCaps(t *testing.T) {
	d := backoffDuration(200*time.Millisecond, 10)
	if d != 5*time.Second {
		t.Fatalf("got %s", d)
	}
	if backoffDuration(200*time.Millisecond, 1) != 200*time.Millisecond {
		t.Fatal("attempt 1")
	}
}
