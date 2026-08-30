package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryAllowsThenRejects(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	if !m.Allow(ctx, "login", "1.1.1.1", 2, time.Minute) {
		t.Fatal("first")
	}
	if !m.Allow(ctx, "login", "1.1.1.1", 2, time.Minute) {
		t.Fatal("second")
	}
	if m.Allow(ctx, "login", "1.1.1.1", 2, time.Minute) {
		t.Fatal("third should 429")
	}
	if !m.Allow(ctx, "login", "2.2.2.2", 2, time.Minute) {
		t.Fatal("other id")
	}
}
