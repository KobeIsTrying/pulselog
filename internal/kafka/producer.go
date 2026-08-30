package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

// Producer publishes canonical log events to a Kafka topic.
type Producer struct {
	writer  *kafkago.Writer
	brokers []string
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		brokers: brokers,
		writer: &kafkago.Writer{
			Addr:                   kafkago.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafkago.Hash{},
			RequiredAcks:           kafkago.RequireOne,
			Compression:            kafkago.Snappy,
			AllowAutoTopicCreation: false,
			BatchTimeout:           10 * time.Millisecond,
			MaxAttempts:            3,
		},
	}
}

func (p *Producer) Publish(ctx context.Context, events []models.LogEvent) error {
	if len(events) == 0 {
		return nil
	}
	msgs := make([]kafkago.Message, 0, len(events))
	for _, ev := range events {
		body, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		msgs = append(msgs, kafkago.Message{
			Key:   []byte(ev.Service),
			Value: body,
			Time:  ev.Timestamp,
			Headers: []kafkago.Header{
				{Key: "event_id", Value: []byte(ev.EventID)},
				{Key: "level", Value: []byte(ev.Level)},
				{Key: "project_id", Value: []byte(ev.ProjectID)},
			},
		})
	}
	start := time.Now()
	err := p.writer.WriteMessages(ctx, msgs...)
	metrics.KafkaProduceDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.KafkaProduceErrors.Inc()
		return fmt.Errorf("kafka write: %w", err)
	}
	return nil
}

func (p *Producer) Ready(ctx context.Context) error {
	if len(p.brokers) == 0 {
		return fmt.Errorf("no kafka brokers configured")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return fmt.Errorf("kafka dial: %w", err)
	}
	defer conn.Close()
	return nil
}

func (p *Producer) Close() error {
	if p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
