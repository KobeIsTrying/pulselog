# Delivery phases

Work is incremental. Each phase leaves the repo compilable, documented, and testable.

## MVP (Phases 1–5)

The MVP is a local, Docker-composed platform that can ingest logs, persist them, query them, and visualize them with live updates and login.

### Phase 1 — Foundation and ingestion (complete)

- Monorepo layout, README, env contract
- Docker Compose: Kafka (KRaft), ClickHouse, PostgreSQL, Redis
- ClickHouse and PostgreSQL schemas
- Shared Go packages: config, logger, models, Kafka producer, HTTP middleware, metrics
- `ingestion-api`: validate → Kafka, health, readiness, Prometheus
- Unit tests for validation and HTTP handlers
- GitHub Actions (`go test`, `gofmt`, compose config)

### Phase 2 — Log processor (complete)

- Kafka consumer group `log-processor`
- Batch inserts into ClickHouse
- Retry + dead-letter topic `logs-dlq`
- Processor metrics and `/healthz` `/readyz` on `:8081`
- Unit tests with mocked store/DLQ

### Phase 3 — Query API (complete)

- Search / filter / keyset pagination
- Service stats, time-series, common ERROR messages
- Query API tests with a mocked store
- Auth (JWT) deferred to a later phase as requested

### Phase 4 — Authentication, API keys, and abuse protection (complete)

- PostgreSQL identity: orgs, projects, services, memberships, API keys, audit
- JWT login/logout (Redis denylist) and argon2id passwords
- Ingest API keys (`pl_live_...`) stored as SHA-256 only
- Query API scoped by authorized `project_id`
- Redis rate limits on login, ingest, and query
- Auth/authz/rate-limit metrics

### Phase 5 — Dashboard (complete)

- Next.js App Router + TypeScript + Tailwind + Recharts
- BFF auth (httpOnly JWT cookie); login / signup against the Query API
- Overview, log explorer, services, projects, API keys
- Manual refresh and optional 10s/30s/60s polling (no WebSockets)
- Frontend unit tests
- Real ingest → Kafka → processor → ClickHouse → Query API → dashboard verification
- Stats endpoints honor `project_id` the same way list-logs already did

### Phase 6 — Real-time streaming (complete)

- Processor publishes `log.created` to Redis `pulselog:logs:{project_id}` only after ClickHouse insert succeeds
- Query API `GET /api/v1/stream` (ticket or Bearer) + `POST /api/v1/stream/ticket`
- Server-side `HasProject` before subscribe; Redis pub/sub fanout across API instances
- Dashboard LIVE mode: filters, `event_id` dedupe, pause/resume, reconnect backoff, REST fallback
- Overview increments total/error/warn then debounces REST chart/service refresh
- Tests for authz, hub isolation, processor fanout timing, and frontend live helpers

### Phase 7 — Performance, reliability, observability (complete)

- k6 scenarios in `tests/load` (ingest, query, mixed, WebSocket, rate-limit)
- Synthetic logs with `bench:{RUN_ID}` and per-service API keys
- Measured local ingest/query/WS/mixed baselines and five failure/recovery checks
- Metrics: Kafka consumer lag gauge, WebSocket drop counter, `WS_HUB_BUFFER`
- Optional Compose profile `obs`: Prometheus `:9090`, Grafana `:3001`
- Report: [docs/PERFORMANCE.md](PERFORMANCE.md)

### Phase 8 — Containerization, CI/CD, production hardening (complete)

- Production Dockerfiles: ingestion-api, log-processor, query-api, dashboard (standalone), migrate
- Production-like Compose (`infrastructure/docker-compose.prod.yml`) with healthchecks, migrate job, split networks
- Configurable CORS / cookie Secure / `AUTH_SIGNUPS` / ClickHouse TTL
- Advisory-locked PostgreSQL migrations; deterministic ClickHouse schema + Kafka topic init
- GitHub Actions CI + GHCR publish; Kubernetes app manifests
- Guide: [docs/DEPLOYMENT.md](DEPLOYMENT.md)

Do not provision paid cloud resources until this phase is approved.

## Later (beyond MVP)

- Horizontal Pod Autoscaler
- OpenTelemetry traces across ingest → process → query
- Multi-broker Kafka and ClickHouse replication operations
- S3/object-storage cold tier for expired partitions
- Alerting rules (error-rate SLO burn)
- Schema registry / protobuf events

