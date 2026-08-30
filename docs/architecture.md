# PulseLog Architecture

PulseLog is a high-throughput distributed log aggregator. Applications emit structured events over HTTP. The ingestion API validates them and publishes to Kafka. A worker consumes Kafka, batches writes into ClickHouse, and failed poison messages land on a dead-letter topic. A query API serves search, aggregations, and WebSocket live updates to a Next.js dashboard.

Production-style deploy, networking, and secrets: [DEPLOYMENT.md](DEPLOYMENT.md).

## System diagram

```
                    ┌─────────────────────────────────────────────────────────┐
                    │                         Clients                         │
                    │   apps / servers / k6 load test / dashboard browsers    │
                    └───────────────┬─────────────────────────┬───────────────┘
                                    │ POST /v1/logs           │ REST + WS + JWT
                                    ▼                         ▼
                          ┌─────────────────┐       ┌─────────────────┐
                          │  ingestion-api  │       │    query-api    │
                          │  Go / REST      │       │  Go / REST+WS   │
                          │  validate+pub   │       │  search/stats   │
                          └────────┬────────┘       └────────┬────────┘
                                   │ produce                 │ query / subscribe
                                   ▼                         │
                          ┌─────────────────┐                │
                          │  Apache Kafka   │                │
                          │  logs-ingest    │                │
                          │  logs-dlq       │                │
                          └────────┬────────┘                │
                                   │ consume                 │
                                   ▼                         │
                          ┌─────────────────┐                │
                          │  log-processor  │──writes──┐     │
                          │  Go worker      │          │     │
                          │  batch + retry  │          ▼     ▼
                          └────────┬────────┘     ┌─────────────────┐
                                   │ live fanout  │   ClickHouse    │
                                   ▼              │   logs + MVs    │
                          ┌─────────────────┐     └─────────────────┘
                          │      Redis      │
                          │ cache + pub/sub │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐     ┌─────────────────┐
                          │    query-api    │────►│   PostgreSQL    │
                          │  WS hub         │     │  users / meta   │
                          └─────────────────┘     └─────────────────┘
```

## Data flow

1. **Client → Ingestion API**  
   A service POSTs a JSON event (or a batch) to `POST /v1/logs` or `POST /v1/logs/batch`. The API validates schema, size, and field constraints. Invalid requests fail fast with `400`. The process never writes ClickHouse on the ingest path.

2. **Ingestion API → Kafka**  
   Valid events are assigned an `event_id`, normalized (level uppercased, timestamp defaulted), and published to `logs-ingest`. The Kafka key is the service name so a given service’s events stay ordered within a partition. Kafka unavailability returns `503`.

3. **Kafka → Log Processor**  
   The worker is consumer group `log-processor`. It fetches messages, unmarshals them, sends permanently malformed payloads to `logs-dlq`, retries transient ClickHouse errors with bounded backoff, and dead-letters a batch only after retries are exhausted. Kafka offsets are committed only after a successful ClickHouse write or a successful DLQ publish.

4. **Log Processor → ClickHouse**  
   Events are inserted in batches (size and/or time) into `pulselog.logs`. Materialized views maintain per-minute counters for cheap dashboard charts. Delivery is at-least-once; `event_id` is de-duplicated only within a processor batch.

5. **Log Processor → Redis (Phase 6)**  
   After a successful insert, a compact live payload is published on Redis channel `pulselog:logs:{project_id}` so all query-api replicas can fan out to WebSocket clients. Redis failure does not block the Kafka commit.

6. **Query API → ClickHouse / PostgreSQL / Redis**  
   Dashboard search, filters, pagination, and aggregations hit ClickHouse. Auth and user metadata live in PostgreSQL. Hot stats and live streams use Redis.

7. **Query API → Dashboard**  
   Next.js renders charts and the log table over REST. An optional WebSocket receives `log.created` frames for the authorized project. Polling remains the fallback.

## Kafka topics

| Topic | Partitions (dev) | Purpose |
| --- | --- | --- |
| `logs-ingest` | 6 | Validated events from the ingestion API |
| `logs-dlq` | 3 | Permanently failed processor messages |

Production would raise partition counts with throughput, keep replication factor ≥ 3, and enable compaction only on metadata topics (not on log streams).

## Storage design

**ClickHouse `logs`** is a `MergeTree` table partitioned by day, ordered by `(service, level, timestamp, event_id)` so the common dashboard filters prune granules. `event_id` is a UUID assigned at ingest and preserved through Kafka, the processor, DLQ, and ClickHouse. A `tokenbf_v1` skip index on `message` supports cheap token search. TTL drops raw logs after 90 days. Delivery is at-least-once: a crash between INSERT and Kafka commit can duplicate a row with the same `event_id`. Query `LIMIT 1 BY event_id`, or later switch the engine to `ReplacingMergeTree(ingested_at)` with the same ORDER BY.

**ClickHouse `logs_per_minute`** is a `SummingMergeTree` fed by a materialized view. Charts read this table instead of scanning raw logs.

**PostgreSQL** holds dashboard users and (later) API tokens / service registry.

**Redis** caches expensive aggregations and fans out live events.

## Service boundaries

| Service | Responsibility | Does not |
| --- | --- | --- |
| `ingestion-api` | Auth-optional ingest, validation, Kafka produce, `/healthz` `/readyz` `/metrics` | Query ClickHouse, serve UI |
| `log-processor` | Consume, batch, ClickHouse write, DLQ, live publish | Expose public HTTP (except optional admin health) |
| `query-api` | Search, stats, JWT auth, org/project admin | Accept raw ingest |
| `dashboard` | Visualization and login | Talk to Kafka or ClickHouse directly |

## Observability

- Structured JSON logs via `log/slog`
- Prometheus metrics on `/metrics` (request latency, ingest counts, Kafka produce errors, later consumer lag and CH write latency)
- Liveness: `/healthz` (process up)
- Readiness: `/readyz` (ingest: Kafka; processor: ClickHouse; query-api: ClickHouse + PostgreSQL + Redis PING). A dead realtime subscription is retried and does not fail REST.

## Failure handling

| Failure | Behavior |
| --- | --- |
| Invalid HTTP payload | HTTP 400; never enters Kafka |
| Kafka down (ingest) | HTTP 503; client retries |
| Malformed Kafka message | Processor → `logs-dlq` (`invalid_json` / `validation_failed`) |
| ClickHouse timeout | Processor retries with backoff; after 5 attempts → `logs-dlq` |
| DLQ publish failure | Offsets are not committed; batch is retried |
| Query API down | Ingest and processing continue; dashboard degrades |

## Trust boundaries

- Ingest is a high-volume untrusted input path: strict validation, body limits, no secrets in responses.
- Query API is authenticated with JWT. Ingest uses hashed API keys. The dashboard stores the JWT in an httpOnly cookie and proxies `/api/backend/*` to query-api.
- All credentials come from environment variables. Compose defaults are **local-only**.
