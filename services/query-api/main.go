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

	"github.com/pulselog/pulselog/internal/auth"
	ch "github.com/pulselog/pulselog/internal/clickhouse"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/logger"
	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/postgres"
	"github.com/pulselog/pulselog/internal/ratelimit"
	"github.com/pulselog/pulselog/internal/realtime"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("query-api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("query-api")
	if err != nil {
		return err
	}
	log := logger.New(cfg.ServiceName, cfg.Env, cfg.LogLevel)
	metrics.Register()

	ctx := context.Background()
	client, err := ch.OpenClient(cfg.ClickHouse)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			log.Error("clickhouse close", "err", cerr)
		}
	}()
	if err := client.EnsureSchema(ctx, cfg.ClickHouse); err != nil {
		return err
	}

	pool, err := postgres.Open(ctx, cfg.Postgres.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	defer func() { _ = rdb.Close() }()

	tokens, err := auth.NewIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTTTL)
	if err != nil {
		return err
	}
	deny := auth.RedisDeny{
		Set: func(c context.Context, key string, ttl time.Duration) error {
			return rdb.Set(c, key, "1", ttl).Err()
		},
		Exists: func(c context.Context, key string) (bool, error) {
			n, err := rdb.Exists(c, key).Result()
			return n > 0, err
		},
	}

	srv := NewServer(log, newCHStore(client), cfg.Query.Timeout)
	srv.dir = identity.New(pool)
	srv.tokens = tokens
	srv.deny = deny
	srv.limit = ratelimit.New(rdb)
	srv.signups = cfg.Auth.AllowSignup
	srv.cors = cfg.Auth.CORSOrigins
	srv.env = cfg.Env
	srv.rate = cfg.RateLimit
	srv.rdb = rdb
	srv.hub = realtime.NewHub()
	srv.wsBuffer = cfg.Query.WSBuffer
	// Redis PING is required for JWT denylist and rate limits. A dead pub/sub
	// subscription does not fail this check; the subscriber retries in-process.
	srv.readies = []func(context.Context) error{
		func(c context.Context) error { return srv.dir.Ping(c) },
		func(c context.Context) error { return rdb.Ping(c).Err() },
	}

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		// WebSocket pumps manage their own deadlines. A global write timeout
		// would tear down live connections after 20s.
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  90 * time.Second,
	}

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go realtime.NewSubscriber(rdb, srv.hub, log).RunLogged(runCtx)

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr, "clickhouse", cfg.ClickHouse.Addr, "query_timeout", cfg.Query.Timeout)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-runCtx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
