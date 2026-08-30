package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/redis/go-redis/v9"
)

// Gate is implemented by the Redis limiter and the in-memory test double.
type Gate interface {
	Allow(ctx context.Context, scope, id string, limit int, window time.Duration) bool
}

// Limiter is a Redis fixed-window counter. Keys expire with the window.
type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

var incrExpire = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return n
`)

func (l *Limiter) Allow(ctx context.Context, scope, id string, limit int, window time.Duration) bool {
	if l == nil || l.rdb == nil || limit <= 0 {
		return true
	}
	if window <= 0 {
		window = time.Minute
	}
	slot := time.Now().UTC().Unix() / int64(window.Seconds())
	key := fmt.Sprintf("rl:%s:%s:%d", scope, id, slot)
	n, err := incrExpire.Run(ctx, l.rdb, []string{key}, int(window.Seconds())+1).Int()
	if err != nil {
		return true
	}
	if n > limit {
		metrics.RateLimited.WithLabelValues(scope).Inc()
		return false
	}
	return true
}

type Memory struct {
	counts map[string]int
}

func NewMemory() *Memory {
	return &Memory{counts: map[string]int{}}
}

func (m *Memory) Allow(_ context.Context, scope, id string, limit int, _ time.Duration) bool {
	if m == nil || limit <= 0 {
		return true
	}
	key := scope + ":" + id
	m.counts[key]++
	if m.counts[key] > limit {
		metrics.RateLimited.WithLabelValues(scope).Inc()
		return false
	}
	return true
}
