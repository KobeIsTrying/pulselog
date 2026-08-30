package realtime

import (
	"testing"
	"time"

	"github.com/pulselog/pulselog/internal/models"
)

func TestChannelIsolation(t *testing.T) {
	a := Channel("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	b := Channel("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	if a == b {
		t.Fatal("channels collided")
	}
	if ProjectFromChannel(a) != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("project from channel: %q", ProjectFromChannel(a))
	}
}

func TestEncodeDecodeEvent(t *testing.T) {
	ev := models.LogEvent{
		EventID:   "11111111-1111-4111-8111-111111111111",
		ProjectID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		Service:   "payment-service",
		Level:     "ERROR",
		Message:   "card declined",
		Timestamp: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC),
		Metadata:  map[string]string{"k": "v"},
	}
	raw, err := EncodeEvent(ev)
	if err != nil {
		t.Fatal(err)
	}
	env, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != TypeLogCreated || env.V != 1 {
		t.Fatalf("%+v", env)
	}
	if env.Data.EventID != ev.EventID || env.Data.Service != ev.Service || env.Data.TraceID != "" {
		t.Fatalf("%+v", env.Data)
	}
}

func TestHubProjectIsolationAndFanout(t *testing.T) {
	h := NewHub()
	a1 := NewClient("proj-a", 8)
	a2 := NewClient("proj-a", 8)
	b1 := NewClient("proj-b", 8)
	h.Add(a1)
	h.Add(a2)
	h.Add(b1)
	if h.Deliver("proj-a", []byte(`{"v":1}`)) != 2 {
		t.Fatal("expected two subscribers on proj-a")
	}
	if h.Deliver("proj-b", []byte(`{"v":1}`)) != 1 {
		t.Fatal("expected one subscriber on proj-b")
	}
	select {
	case <-a1.Send:
	default:
		t.Fatal("a1 missed")
	}
	select {
	case <-b1.Send:
	default:
		t.Fatal("b1 missed")
	}
	h.Remove(a1)
	h.Remove(a2)
	h.Remove(b1)
	if h.Count() != 0 {
		t.Fatalf("leaked clients: %d", h.Count())
	}
}

func TestHubDropsWhenBufferFull(t *testing.T) {
	h := NewHub()
	c := NewClient("proj-drop", 1)
	h.Add(c)
	if n := h.Deliver("proj-drop", []byte("one")); n != 1 {
		t.Fatalf("first deliver = %d", n)
	}
	if n := h.Deliver("proj-drop", []byte("two")); n != 0 {
		t.Fatalf("expected drop, got %d", n)
	}
	h.Remove(c)
}

func TestMemoryPublisherSkipsLegacyProject(t *testing.T) {
	pub := &MemoryPublisher{}
	_ = pub.Publish(nil, []models.LogEvent{
		{EventID: "1", ProjectID: "", Service: "x", Level: "INFO", Message: "m"},
		{EventID: "2", ProjectID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Service: "x", Level: "INFO", Message: "m"},
	})
	if len(pub.Messages) != 1 {
		t.Fatalf("%d messages", len(pub.Messages))
	}
}
