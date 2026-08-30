package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ch "github.com/pulselog/pulselog/internal/clickhouse"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/kafka"
	"github.com/pulselog/pulselog/internal/logger"
	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/realtime"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("log-processor exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("log-processor")
	if err != nil {
		return err
	}
	log := logger.New(cfg.ServiceName, cfg.Env, cfg.LogLevel)
	metrics.Register()

	writer, err := ch.Open(cfg.ClickHouse)
	if err != nil {
		return err
	}
	if err := writer.EnsureSchema(context.Background(), cfg.ClickHouse); err != nil {
		return err
	}
	defer func() {
		if cerr := writer.Close(); cerr != nil {
			log.Error("clickhouse close", "err", cerr)
		}
	}()

	dlq := kafka.NewDLQWriter(cfg.Kafka.Brokers, cfg.Kafka.TopicDLQ)
	defer func() {
		if cerr := dlq.Close(); cerr != nil {
			log.Error("dlq writer close", "err", cerr)
		}
	}()

	reader := kafka.NewReader(cfg.Kafka.Brokers, cfg.Kafka.TopicIngest, cfg.Kafka.ConsumerGroup)
	defer func() {
		if cerr := reader.Close(); cerr != nil {
			log.Error("kafka reader close", "err", cerr)
		}
	}()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Warn("redis unavailable; realtime fanout disabled", "err", err)
	}

	proc := NewProcessor(log, writer, dlq, cfg.Kafka.TopicIngest, cfg.Processor.MaxAttempts, cfg.Processor.RetryBackoff)
	proc.fanout = realtime.NewRedisPublisher(rdb)
	consumer := NewConsumer(reader, proc, cfg.Processor.BatchSize, cfg.Processor.BatchTimeout, cfg.Processor.ShutdownGrace)

	producer := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.TopicIngest)
	defer func() { _ = producer.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newHandler(log, readiness{kafka: producer.Ready, ch: writer.Ping}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		log.Info("listening",
			"addr", cfg.HTTPAddr,
			"kafka_brokers", cfg.Kafka.Brokers,
			"topic", cfg.Kafka.TopicIngest,
			"group", cfg.Kafka.ConsumerGroup,
			"dlq", cfg.Kafka.TopicDLQ,
			"clickhouse", cfg.ClickHouse.Addr,
		)
		err := httpSrv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics.KafkaConsumerLag.Set(float64(reader.Stats().Lag))
			}
		}
	}()
	go func() {
		errCh <- consumer.Run(ctx)
	}()

	select {
	case err := <-errCh:
		stop()
		shutdownHTTP(httpSrv)
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownHTTP(httpSrv)
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		case <-time.After(cfg.Processor.ShutdownGrace + 2*time.Second):
			return nil
		}
	}
}

func shutdownHTTP(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
