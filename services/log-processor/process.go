package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/pulselog/pulselog/internal/kafka"
	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

type Store interface {
	Insert(ctx context.Context, events []models.LogEvent) error
}

type DLQ interface {
	Publish(ctx context.Context, letters []kafka.DeadLetter) error
}

type Fanout interface {
	Publish(ctx context.Context, events []models.LogEvent) error
}

type Processor struct {
	log         *slog.Logger
	store       Store
	dlq         DLQ
	fanout      Fanout
	sourceTopic string
	maxAttempts int
	backoff     time.Duration
	sleep       func(time.Duration)
	now         func() time.Time
}

func NewProcessor(log *slog.Logger, store Store, dlq DLQ, sourceTopic string, maxAttempts int, backoff time.Duration) *Processor {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &Processor{
		log:         log,
		store:       store,
		dlq:         dlq,
		sourceTopic: sourceTopic,
		maxAttempts: maxAttempts,
		backoff:     backoff,
		sleep:       time.Sleep,
		now:         time.Now,
	}
}

// Process classifies a Kafka batch, writes valid events to ClickHouse with retries,
// dead-letters poison and exhausted records, and returns nil only when offsets may be committed.
func (p *Processor) Process(ctx context.Context, msgs []kafkago.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	metrics.ProcessorConsumed.Add(float64(len(msgs)))
	metrics.ProcessorBatchSize.Observe(float64(len(msgs)))

	now := p.now().UTC()
	valid := make([]models.LogEvent, 0, len(msgs))
	validIdx := make([]int, 0, len(msgs))
	letters := make([]kafka.DeadLetter, 0)
	seen := make(map[string]struct{}, len(msgs))

	for i, msg := range msgs {
		topic := msg.Topic
		if topic == "" {
			topic = p.sourceTopic
			msg.Topic = topic
			msgs[i].Topic = topic
		}
		ev, err := models.ParseJSON(msg.Value, now)
		if err != nil {
			reason := kafka.ReasonInvalidJSON
			var ve *models.ValidationError
			if errors.As(err, &ve) {
				reason = kafka.ReasonValidation
			}
			metrics.ProcessorFailed.WithLabelValues(reason).Inc()
			var parsed *models.LogEvent
			if ev.Service != "" || ev.EventID != "" {
				parsed = &ev
			}
			letters = append(letters, kafka.NewDeadLetter(msg, reason, err.Error(), 1, parsed))
			continue
		}
		if _, dup := seen[ev.EventID]; dup {
			p.log.Info("skipping duplicate event_id in batch", "event_id", ev.EventID, "offset", msg.Offset)
			continue
		}
		seen[ev.EventID] = struct{}{}
		valid = append(valid, ev)
		validIdx = append(validIdx, i)
	}

	if len(valid) > 0 {
		if err := p.insertWithRetry(ctx, valid); err != nil {
			metrics.ProcessorFailed.WithLabelValues(kafka.ReasonClickHouse).Add(float64(len(valid)))
			for i, idx := range validIdx {
				ev := valid[i]
				letters = append(letters, kafka.NewDeadLetter(msgs[idx], kafka.ReasonClickHouse, err.Error(), p.maxAttempts, &ev))
			}
		} else {
			metrics.ProcessorWritten.Add(float64(len(valid)))
			p.publishLive(ctx, valid)
		}
	}

	if err := p.dlq.Publish(ctx, letters); err != nil {
		return err
	}
	for _, letter := range letters {
		metrics.ProcessorDLQ.WithLabelValues(letter.Reason).Inc()
	}
	return nil
}

func (p *Processor) publishLive(ctx context.Context, events []models.LogEvent) {
	if p.fanout == nil {
		return
	}
	if err := p.fanout.Publish(ctx, events); err != nil {
		p.log.Warn("realtime publish failed after clickhouse write", "err", err, "n", len(events))
	}
}

func (p *Processor) insertWithRetry(ctx context.Context, events []models.LogEvent) error {
	var last error
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = p.store.Insert(ctx, events)
		if last == nil {
			return nil
		}
		if attempt < p.maxAttempts {
			metrics.ProcessorRetried.Add(float64(len(events)))
			p.log.Warn("clickhouse insert failed, retrying",
				"attempt", attempt,
				"max_attempts", p.maxAttempts,
				"err", last,
				"batch_size", len(events),
			)
			p.sleep(backoffDuration(p.backoff, attempt))
		}
	}
	return last
}

func backoffDuration(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	d := base * time.Duration(1<<uint(attempt-1))
	const capDelay = 5 * time.Second
	if d > capDelay {
		return capDelay
	}
	return d
}
