package main

import (
	"net/url"
	"testing"
	"time"
)

func TestParseListQueryFilters(t *testing.T) {
	q, err := parseListQuery(url.Values{
		"service":   []string{"payment-service"},
		"level":     []string{"error"},
		"q":         []string{"auth failed"},
		"page_size": []string{"10"},
		"start":     []string{"2026-08-29T00:00:00Z"},
		"end":       []string{"2026-08-29T12:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q.Service != "payment-service" || q.Level != "ERROR" || q.Q != "auth failed" || q.Limit != 10 {
		t.Fatalf("%+v", q)
	}
}

func TestParseListQueryRejectsBadLevelAndSize(t *testing.T) {
	if _, err := parseListQuery(url.Values{"level": []string{"TRACE"}}); err == nil {
		t.Fatal("level")
	}
	if _, err := parseListQuery(url.Values{"page_size": []string{"9999"}}); err == nil {
		t.Fatal("page_size")
	}
	if _, err := parseListQuery(url.Values{"service": []string{"bad service"}}); err == nil {
		t.Fatal("service")
	}
	if _, err := parseListQuery(url.Values{"order": []string{"timestamp;drop"}}); err == nil {
		t.Fatal("order")
	}
	if _, err := parseListQuery(url.Values{"start": []string{"nope"}}); err == nil {
		t.Fatal("start")
	}
	if _, err := parseListQuery(url.Values{"start": []string{"2026-08-29T12:00:00Z"}, "end": []string{"2026-08-29T00:00:00Z"}}); err == nil {
		t.Fatal("end before start")
	}
}

func TestParseListQueryEventID(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	q, err := parseListQuery(url.Values{"event_id": []string{id}})
	if err != nil {
		t.Fatal(err)
	}
	if q.EventID != id {
		t.Fatalf("%q", q.EventID)
	}
	if _, err := parseListQuery(url.Values{"event_id": []string{"not-a-uuid"}}); err == nil {
		t.Fatal("event_id")
	}
	if _, err := parseListQuery(url.Values{"page_size": []string{"0"}}); err == nil {
		t.Fatal("page_size 0")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 29, 12, 0, 0, 123000000, time.UTC)
	id := "11111111-1111-4111-8111-111111111111"
	enc := encodeCursor(ts, id)
	got, err := decodeCursor(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TS.Equal(ts) || got.ID != id {
		t.Fatalf("%+v", got)
	}
	if _, err := decodeCursor("%%%"); err == nil {
		t.Fatal("invalid cursor")
	}
	if _, err := parseListQuery(url.Values{"cursor": []string{"not-a-cursor"}}); err == nil {
		t.Fatal("cursor param")
	}
}

func TestParseTimeseriesIntervalAllowlist(t *testing.T) {
	q, err := parseTimeseriesQuery(url.Values{"interval": []string{"5m"}})
	if err != nil {
		t.Fatal(err)
	}
	if q.Interval != "5m" {
		t.Fatalf("%s", q.Interval)
	}
	if _, err := parseTimeseriesQuery(url.Values{"interval": []string{"1s; SELECT 1"}}); err == nil {
		t.Fatal("bad interval")
	}
}

func TestParseServiceSortAllowlist(t *testing.T) {
	q, err := parseServiceStatsQuery(url.Values{"sort": []string{"total"}})
	if err != nil {
		t.Fatal(err)
	}
	if q.Sort != "total" {
		t.Fatal(q.Sort)
	}
	if _, err := parseServiceStatsQuery(url.Values{"sort": []string{"errors DESC;drop"}}); err == nil {
		t.Fatal("bad sort")
	}
}
