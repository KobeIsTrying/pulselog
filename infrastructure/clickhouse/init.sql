-- PulseLog ClickHouse schema.
--
-- Engine: MergeTree — default analytics engine; parts merge in the background.
-- Partition: toYYYYMMDD(timestamp) — time-range queries drop whole days.
-- ORDER BY (service, level, timestamp, event_id) — dashboard filters
--   (service + level + time) prune granules; event_id breaks ties and keeps
--   duplicates adjacent for LIMIT 1 BY event_id.
-- TTL 90 days on raw logs. logs_per_minute is a SummingMergeTree MV for charts.
--
-- Idempotency: MergeTree has no unique constraint. Kafka delivery is
-- at-least-once, so a crash between a successful INSERT and the Kafka
-- offset commit can produce a second row with the same event_id.
-- event_id is a UUID in the ORDER BY so:
--   * LIMIT 1 BY event_id can collapse duplicates at query time
--   * a later ReplacingMergeTree(ingested_at) with this same ORDER BY
--     can merge exact-key duplicates in the background
-- The processor de-duplicates event_id only within a single batch.
-- This is not exactly-once delivery.

CREATE DATABASE IF NOT EXISTS pulselog;

CREATE TABLE IF NOT EXISTS pulselog.logs
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
TTL toDateTime(timestamp) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Pre-aggregated series for dashboard charts (logs / errors per minute).
CREATE TABLE IF NOT EXISTS pulselog.logs_per_minute
(
    minute  DateTime('UTC'),
    service LowCardinality(String),
    level   LowCardinality(String),
    count   UInt64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(minute)
ORDER BY (minute, service, level)
TTL minute + INTERVAL 180 DAY;

CREATE MATERIALIZED VIEW IF NOT EXISTS pulselog.logs_per_minute_mv
TO pulselog.logs_per_minute
AS
SELECT
    toStartOfMinute(timestamp) AS minute,
    service,
    level,
    count() AS count
FROM pulselog.logs
GROUP BY
    minute,
    service,
    level;
