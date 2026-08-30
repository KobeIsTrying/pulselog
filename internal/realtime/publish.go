package realtime

import (
	"context"

	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/models"
	"github.com/redis/go-redis/v9"
)

type Publisher interface {
	Publish(ctx context.Context, events []models.LogEvent) error
}

type RedisPublisher struct {
	rdb *redis.Client
}

func NewRedisPublisher(rdb *redis.Client) *RedisPublisher {
	return &RedisPublisher{rdb: rdb}
}

func (p *RedisPublisher) Publish(ctx context.Context, events []models.LogEvent) error {
	if p == nil || p.rdb == nil || len(events) == 0 {
		return nil
	}
	byProject := map[string][][]byte{}
	for _, ev := range events {
		if ev.ProjectID == "" || ev.ProjectID == "00000000-0000-0000-0000-000000000000" {
			continue
		}
		payload, err := EncodeEvent(ev)
		if err != nil {
			metrics.RealtimePublishErrors.Inc()
			continue
		}
		byProject[ev.ProjectID] = append(byProject[ev.ProjectID], payload)
	}
	var last error
	for projectID, payloads := range byProject {
		ch := Channel(projectID)
		for _, payload := range payloads {
			if err := p.rdb.Publish(ctx, ch, payload).Err(); err != nil {
				metrics.RealtimePublishErrors.Inc()
				last = err
				continue
			}
			metrics.RealtimePublished.Inc()
		}
	}
	return last
}

type MemoryPublisher struct {
	Messages []Published
}

type Published struct {
	Channel string
	Payload []byte
}

func (m *MemoryPublisher) Publish(_ context.Context, events []models.LogEvent) error {
	for _, ev := range events {
		if ev.ProjectID == "" {
			continue
		}
		payload, err := EncodeEvent(ev)
		if err != nil {
			return err
		}
		m.Messages = append(m.Messages, Published{Channel: Channel(ev.ProjectID), Payload: payload})
	}
	return nil
}
