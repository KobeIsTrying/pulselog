package kafka

import (
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

func NewReader(brokers []string, topic, group string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        brokers,
		GroupID:        group,
		Topic:          topic,
		StartOffset:    kafkago.FirstOffset,
		MinBytes:       1,
		MaxBytes:       10 << 20,
		MaxWait:        250 * time.Millisecond,
		CommitInterval: 0,
	})
}
