package realtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type Subscriber struct {
	rdb *redis.Client
	hub *Hub
	log *slog.Logger
}

func NewSubscriber(rdb *redis.Client, hub *Hub, log *slog.Logger) *Subscriber {
	return &Subscriber{rdb: rdb, hub: hub, log: log}
}

func (s *Subscriber) Run(ctx context.Context) error {
	if s == nil || s.rdb == nil || s.hub == nil {
		return nil
	}
	pubsub := s.rdb.PSubscribe(ctx, ChannelPrefix+"*")
	defer func() { _ = pubsub.Close() }()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if msg == nil {
				continue
			}
			projectID := ProjectFromChannel(msg.Channel)
			if projectID == "" || projectID == msg.Channel {
				continue
			}
			s.hub.Deliver(projectID, []byte(msg.Payload))
		}
	}
}

func (s *Subscriber) RunLogged(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := s.Run(ctx)
		if ctx.Err() != nil {
			return
		}
		metrics.RedisSubscribeErrors.Inc()
		if s.log != nil {
			s.log.Error("realtime subscriber stopped; retrying", "err", err, "backoff", backoff)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 15*time.Second {
			backoff *= 2
		}
	}
}
