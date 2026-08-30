package main

import (
	"context"
	"errors"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type messageSource interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
}

type Consumer struct {
	src           messageSource
	proc          *Processor
	batchSize     int
	batchTimeout  time.Duration
	shutdownGrace time.Duration
}

func NewConsumer(src messageSource, proc *Processor, batchSize int, batchTimeout, shutdownGrace time.Duration) *Consumer {
	if batchSize < 1 {
		batchSize = 1
	}
	if batchTimeout <= 0 {
		batchTimeout = 500 * time.Millisecond
	}
	if shutdownGrace <= 0 {
		shutdownGrace = 15 * time.Second
	}
	return &Consumer{
		src:           src,
		proc:          proc,
		batchSize:     batchSize,
		batchTimeout:  batchTimeout,
		shutdownGrace: shutdownGrace,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	buf := make([]kafkago.Message, 0, c.batchSize)
	var runErr error
	defer func() {
		if runErr != nil || len(buf) == 0 {
			return
		}
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdownGrace)
		defer cancel()
		_ = c.flush(flushCtx, buf)
	}()

	for {
		if ctx.Err() != nil {
			return nil
		}
		fetchCtx, cancel := context.WithTimeout(ctx, c.batchTimeout)
		msg, err := c.src.FetchMessage(fetchCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) && len(buf) > 0 {
				if ferr := c.flush(ctx, buf); ferr != nil {
					runErr = ferr
					return ferr
				}
				buf = buf[:0]
			}
			continue
		}
		buf = append(buf, msg)
		if len(buf) >= c.batchSize {
			if ferr := c.flush(ctx, buf); ferr != nil {
				runErr = ferr
				return ferr
			}
			buf = buf[:0]
		}
	}
}

func (c *Consumer) flush(ctx context.Context, buf []kafkago.Message) error {
	if len(buf) == 0 {
		return nil
	}
	if err := c.proc.Process(ctx, buf); err != nil {
		return err
	}
	return c.src.CommitMessages(ctx, buf...)
}
