package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	ReasonInvalidJSON = "invalid_json"
	ReasonValidation  = "validation_failed"
	ReasonClickHouse  = "clickhouse_write_failed"
	maxDLQRawBytes    = 64 << 10
)

// DeadLetter is the Kafka value written to logs-dlq.
type DeadLetter struct {
	FailedAt        time.Time        `json:"failed_at"`
	Reason          string           `json:"reason"`
	Error           string           `json:"error"`
	Attempts        int              `json:"attempts"`
	SourceTopic     string           `json:"source_topic"`
	SourcePartition int              `json:"source_partition"`
	SourceOffset    int64            `json:"source_offset"`
	Raw             string           `json:"raw"`
	Event           *models.LogEvent `json:"event,omitempty"`
}

type DLQWriter struct {
	writer *kafkago.Writer
	topic  string
}

func NewDLQWriter(brokers []string, topic string) *DLQWriter {
	return &DLQWriter{
		topic: topic,
		writer: &kafkago.Writer{
			Addr:                   kafkago.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafkago.LeastBytes{},
			RequiredAcks:           kafkago.RequireOne,
			Compression:            kafkago.Snappy,
			AllowAutoTopicCreation: false,
			MaxAttempts:            3,
		},
	}
}

func NewDeadLetter(msg kafkago.Message, reason, errMsg string, attempts int, ev *models.LogEvent) DeadLetter {
	raw := string(msg.Value)
	if len(raw) > maxDLQRawBytes {
		raw = raw[:maxDLQRawBytes]
	}
	topic := msg.Topic
	return DeadLetter{
		FailedAt:        time.Now().UTC(),
		Reason:          reason,
		Error:           errMsg,
		Attempts:        attempts,
		SourceTopic:     topic,
		SourcePartition: msg.Partition,
		SourceOffset:    msg.Offset,
		Raw:             raw,
		Event:           ev,
	}
}

func (w *DLQWriter) Publish(ctx context.Context, letters []DeadLetter) error {
	if w == nil || len(letters) == 0 {
		return nil
	}
	msgs := make([]kafkago.Message, 0, len(letters))
	for _, letter := range letters {
		body, err := json.Marshal(letter)
		if err != nil {
			return fmt.Errorf("marshal dlq: %w", err)
		}
		key := []byte("unknown")
		if letter.Event != nil && letter.Event.Service != "" {
			key = []byte(letter.Event.Service)
		}
		msgs = append(msgs, kafkago.Message{
			Key:   key,
			Value: body,
			Time:  letter.FailedAt,
			Headers: []kafkago.Header{
				{Key: "reason", Value: []byte(letter.Reason)},
			},
		})
	}
	start := time.Now()
	err := w.writer.WriteMessages(ctx, msgs...)
	metrics.KafkaProduceDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.KafkaProduceErrors.Inc()
		return fmt.Errorf("kafka dlq write: %w", err)
	}
	return nil
}

func (w *DLQWriter) Close() error {
	if w == nil || w.writer == nil {
		return nil
	}
	return w.writer.Close()
}
