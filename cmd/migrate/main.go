package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	ch "github.com/pulselog/pulselog/internal/clickhouse"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/logger"
	"github.com/pulselog/pulselog/internal/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("migrate")
	if err != nil {
		return err
	}
	log := logger.New("migrate", cfg.Env, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := postgres.Open(ctx, cfg.Postgres.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Info("applying postgres migrations")
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}

	client, err := ch.OpenClient(cfg.ClickHouse)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	log.Info("applying clickhouse schema", "ttl_days", cfg.ClickHouse.TTLDays, "mv_ttl_days", cfg.ClickHouse.MVTTLDays)
	if err := client.EnsureSchema(ctx, cfg.ClickHouse); err != nil {
		return err
	}
	log.Info("migrations complete")
	return nil
}
