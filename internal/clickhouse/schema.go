package clickhouse

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pulselog/pulselog/internal/config"
)

func (c *Client) EnsureSchema(ctx context.Context, cfg config.ClickHouseConfig) error {
	if c == nil || c.Conn == nil {
		return fmt.Errorf("clickhouse not connected")
	}
	return ensureSchema(ctx, c.Conn, cfg)
}

func (w *Writer) EnsureSchema(ctx context.Context, cfg config.ClickHouseConfig) error {
	if w == nil || w.conn == nil {
		return fmt.Errorf("clickhouse not connected")
	}
	return ensureSchema(ctx, w.conn, cfg)
}

func ensureSchema(ctx context.Context, conn driver.Conn, cfg config.ClickHouseConfig) error {
	db := cfg.Database
	if db == "" {
		db = "pulselog"
	}
	if !identRE.MatchString(db) {
		return fmt.Errorf("invalid clickhouse database identifier")
	}
	table := cfg.Table
	if table == "" {
		table = "logs"
	}
	if !identRE.MatchString(table) {
		return fmt.Errorf("invalid clickhouse table identifier")
	}
	ttl := cfg.TTLDays
	if ttl < 1 {
		ttl = 90
	}
	mvTTL := cfg.MVTTLDays
	if mvTTL < 1 {
		mvTTL = 180
	}

	stmts := []string{
		fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s`, db),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s
(
    event_id    UUID,
    timestamp   DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),
    service     LowCardinality(String),
    level       LowCardinality(String),
    message     String,
    host        LowCardinality(String),
    trace_id    String,
    metadata    Map(String, String),
    project_id  UUID DEFAULT toUUID('00000000-0000-0000-0000-000000000000'),
    INDEX idx_message_tokens message TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 4
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (service, level, timestamp, event_id)
TTL toDateTime(timestamp) + INTERVAL %d DAY
SETTINGS index_granularity = 8192`, db, table, ttl),
		fmt.Sprintf(`ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS project_id UUID DEFAULT toUUID('00000000-0000-0000-0000-000000000000')`, db, table),
		fmt.Sprintf(`ALTER TABLE %s.%s MODIFY TTL toDateTime(timestamp) + INTERVAL %d DAY`, db, table, ttl),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.logs_per_minute
(
    minute  DateTime('UTC'),
    service LowCardinality(String),
    level   LowCardinality(String),
    count   UInt64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(minute)
ORDER BY (minute, service, level)
TTL minute + INTERVAL %d DAY`, db, mvTTL),
		fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s.logs_per_minute_mv
TO %s.logs_per_minute
AS
SELECT
    toStartOfMinute(timestamp) AS minute,
    service,
    level,
    count() AS count
FROM %s.%s
GROUP BY
    minute,
    service,
    level`, db, db, db, table),
		fmt.Sprintf(`ALTER TABLE %s.logs_per_minute MODIFY TTL minute + INTERVAL %d DAY`, db, mvTTL),
	}
	for _, stmt := range stmts {
		if err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("clickhouse schema: %w", err)
		}
	}
	return nil
}
