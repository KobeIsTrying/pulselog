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

	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/kafka"
	"github.com/pulselog/pulselog/internal/logger"
	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/postgres"
	"github.com/pulselog/pulselog/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("ingestion-api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("ingestion-api")
	if err != nil {
		return err
	}
	log := logger.New(cfg.ServiceName, cfg.Env, cfg.LogLevel)
	metrics.Register()

	ctx := context.Background()
	pool, err := postgres.Open(ctx, cfg.Postgres.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}
	ids := identity.New(pool)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	defer func() { _ = rdb.Close() }()

	producer := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.TopicIngest)
	defer func() {
		if cerr := producer.Close(); cerr != nil {
			log.Error("kafka producer close", "err", cerr)
		}
	}()

	srv := NewServer(cfg, log, producer, ids, ratelimit.New(rdb),
		func(c context.Context) error { return ids.Ping(c) },
		func(c context.Context) error { return rdb.Ping(c).Err() },
	)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr, "kafka_brokers", cfg.Kafka.Brokers, "topic", cfg.Kafka.TopicIngest)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-runCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		log.Info("shutting down")
		return httpSrv.Shutdown(shutdownCtx)
	}
}
