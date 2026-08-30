package main

import (
	"context"
	"errors"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type stubSource struct {
	msgs    []kafkago.Message
	commits []kafkago.Message
	fail    error
}

func (s *stubSource) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	if err := ctx.Err(); err != nil {
		return kafkago.Message{}, err
	}
	if len(s.msgs) == 0 {
		<-ctx.Done()
		return kafkago.Message{}, ctx.Err()
	}
	m := s.msgs[0]
	s.msgs = s.msgs[1:]
	return m, nil
}

func (s *stubSource) CommitMessages(_ context.Context, msgs ...kafkago.Message) error {
	if s.fail != nil {
		return s.fail
	}
	s.commits = append(s.commits, msgs...)
	return nil
}

func TestConsumerFlushesBySizeAndCommits(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{}
	p := testProcessor(store, dlq)
	src := &stubSource{msgs: []kafkago.Message{
		validMsg("payment-service", "one"),
		validMsg("other", "two"),
	}}
	c := NewConsumer(src, p, 2, 50*time.Millisecond, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = c.Run(ctx)
	if len(store.got) != 2 {
		t.Fatalf("written = %d", len(store.got))
	}
	if len(src.commits) != 2 {
		t.Fatalf("commits = %d", len(src.commits))
	}
}

func TestConsumerDoesNotCommitWhenProcessFails(t *testing.T) {
	store := &stubStore{}
	dlq := &stubDLQ{err: errors.New("dlq down")}
	p := testProcessor(store, dlq)
	src := &stubSource{msgs: []kafkago.Message{{Value: []byte("{")}}}
	c := NewConsumer(src, p, 1, 50*time.Millisecond, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := c.Run(ctx)
	if err == nil {
		t.Fatal("expected process/dlq error")
	}
	if len(src.commits) != 0 {
		t.Fatalf("commits = %d", len(src.commits))
	}
}
