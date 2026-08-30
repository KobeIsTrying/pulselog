package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const ticketPrefix = "ws:ticket:"

var ErrTicket = errors.New("invalid or expired ticket")

type Ticket struct {
	UserID     uuid.UUID   `json:"user_id"`
	Email      string      `json:"email"`
	JTI        string      `json:"jti"`
	ProjectIDs []uuid.UUID `json:"project_ids"`
}

type TicketStore interface {
	Issue(ctx context.Context, t Ticket, ttl time.Duration) (string, error)
	Redeem(ctx context.Context, id string) (Ticket, error)
}

type RedisTickets struct {
	RDB *redis.Client
}

func (r RedisTickets) Issue(ctx context.Context, t Ticket, ttl time.Duration) (string, error) {
	return IssueTicket(ctx, r.RDB, t, ttl)
}

func (r RedisTickets) Redeem(ctx context.Context, id string) (Ticket, error) {
	return RedeemTicket(ctx, r.RDB, id)
}

func IssueTicket(ctx context.Context, rdb *redis.Client, t Ticket, ttl time.Duration) (string, error) {
	if rdb == nil {
		return "", ErrTicket
	}
	if ttl <= 0 {
		ttl = 45 * time.Second
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw[:])
	b, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	if err := rdb.Set(ctx, ticketPrefix+id, b, ttl).Err(); err != nil {
		return "", err
	}
	return id, nil
}

func RedeemTicket(ctx context.Context, rdb *redis.Client, id string) (Ticket, error) {
	var zero Ticket
	if rdb == nil || id == "" {
		return zero, ErrTicket
	}
	raw, err := rdb.GetDel(ctx, ticketPrefix+id).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return zero, ErrTicket
		}
		return zero, err
	}
	if err := json.Unmarshal(raw, &zero); err != nil {
		return Ticket{}, ErrTicket
	}
	return zero, nil
}

type MemoryTickets struct {
	mu sync.Mutex
	m  map[string]Ticket
}

func (t *MemoryTickets) Issue(_ context.Context, ticket Ticket, _ time.Duration) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw[:])
	t.mu.Lock()
	if t.m == nil {
		t.m = map[string]Ticket{}
	}
	t.m[id] = ticket
	t.mu.Unlock()
	return id, nil
}

func (t *MemoryTickets) Redeem(_ context.Context, id string) (Ticket, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	v, ok := t.m[id]
	if !ok {
		return Ticket{}, ErrTicket
	}
	delete(t.m, id)
	return v, nil
}
