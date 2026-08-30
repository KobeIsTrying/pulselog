# PulseLog

High-throughput distributed log aggregator and metrics dashboard.

PulseLog receives structured logs from many services, validates them on a Go ingestion API, publishes them to Apache Kafka, writes them in batches to ClickHouse, and serves search and aggregations through a JWT Query API consumed by a Next.js dashboard.

## Architecture

```
App + API key  →  POST /v1/logs  →  ingestion-api (:8080)  →  logs-ingest
                                                                ↓
                                                         log-processor (:8081)
                                                           ↙            ↘
                                                  ClickHouse          logs-dlq
                                                  pulselog.logs         │
                                                       │                │
                                                       │  Redis PUBLISH │
                                                       ▼                ▼
User (browser)  →  Next.js dashboard (:3000)     query-api (:8082)
                  httpOnly JWT cookie              REST + WebSocket
                       │                                 │
                       └── /api/auth/stream-ticket ──────┘
```

Realtime events are published **after a successful ClickHouse insert**, before the Kafka offset is committed. A Redis publish failure is logged and counted; it does **not** block the offset commit. Historical REST remains the source of truth. Live delivery is best-effort fanout via Redis pub/sub so multiple query-api processes can serve the same project.

The dashboard is a BFF: the browser calls `/api/auth/*` and `/api/backend/*` on Next.js. The JWT is stored in an httpOnly cookie and is never placed on the WebSocket URL. Live mode uses a short-lived one-time ticket.

### Production routing

```
https://pulselog.example.com        →  dashboard :3000
https://api.pulselog.example.com    →  query-api :8082   (REST + WebSocket)
https://ingest.pulselog.example.com →  ingestion-api :8080

TLS terminates at the reverse proxy / load balancer / ingress.
PostgreSQL, Redis, Kafka, ClickHouse, processor :8081, and /metrics stay private.
```

Complete user, developer, and deployment manual: [docs/INSTRUCTION_MANUAL.md](docs/INSTRUCTION_MANUAL.md)  
Full design: [docs/architecture.md](docs/architecture.md)  
Phased delivery: [docs/phases.md](docs/phases.md)  
Environment contract: [docs/environment.md](docs/environment.md)  
Local benchmarks: [docs/PERFORMANCE.md](docs/PERFORMANCE.md)  
Deployment: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)

## Current status

**Phases 1–8 are implemented.** Ingest requires an API key. Query and admin APIs require a JWT. Tenancy is enforced in PostgreSQL and applied as a ClickHouse `project_id` filter. The dashboard visualizes the real Query API and can subscribe to project-scoped live updates. Phase 7 measured a local development baseline (not production capacity). Phase 8 adds production Docker images, production-like Compose, CI/CD, GHCR publishing, Kubernetes manifests for the application services, and a documented cloud-neutral deploy path. No paid cloud resources are provisioned from this repo.

## Kafka topics

| Topic | Purpose |
| --- | --- |
| `logs-ingest` | Validated events from the ingestion API |
| `logs-dlq` | Processor dead letters (malformed, invalid, or ClickHouse exhausted) |

Consumer group: **`log-processor`**. Offsets are committed only after ClickHouse insert (or DLQ publish) succeeds.

## ClickHouse schema (`pulselog.logs`)

| Column | Type | Role |
| --- | --- | --- |
| `event_id` | UUID | Ingest-assigned id |
| `timestamp` | DateTime64(3, UTC) | Event time |
| `ingested_at` | DateTime64(3, UTC) | Processor write time |
| `service` | LowCardinality(String) | Emitting service |
| `level` | LowCardinality(String) | DEBUG…FATAL |
| `message` | String | Body (token bloom index) |
| `host` | LowCardinality(String) | Optional |
| `trace_id` | String | Optional |
| `metadata` | Map(String, String) | Optional |
| `project_id` | UUID | Tenant stamp from the API key (zero UUID on pre-auth rows) |

- **Engine:** `MergeTree`
- **Partition:** `toYYYYMMDD(timestamp)` so time-range queries skip whole days
- **ORDER BY:** `(service, level, timestamp, event_id)` matches dashboard filters
- **TTL:** 90 days on raw logs (`CLICKHOUSE_TTL_DAYS`); `logs_per_minute` is a `SummingMergeTree` materialized view (`CLICKHOUSE_MV_TTL_DAYS`, default 180)

## Processor behavior

- Fetch from `logs-ingest`, flush at **100 events** or **500ms**
- Deserialize + validate; poison messages go to DLQ immediately
- Insert valid events in one ClickHouse batch
- **Retries:** 5 attempts, exponential backoff from 200ms, capped at 5s
- After retries are exhausted, the batch is published to `logs-dlq` (reason `clickhouse_write_failed`) so the partition is not blocked forever
- After a successful insert, each event with a real `project_id` is published to Redis channel `pulselog:logs:{project_id}`
- Redis publish is best-effort: failure does not block the Kafka commit
- Then Kafka offsets for that batch are committed
- If DLQ publish fails, offsets are **not** committed
- In-batch de-duplication by `event_id` only

## Event IDs and delivery semantics

Every accepted event gets a **UUID `event_id`** at ingest (client-supplied if valid, otherwise generated). That id is written into the Kafka value, the `event_id` header, the processor, ClickHouse `event_id`, and DLQ payloads when the event could be parsed. The processor does not mint a new id when one is already present.

Delivery is **at-least-once, not exactly-once.**

Kafka offsets are committed only after ClickHouse insert (or DLQ publish) succeeds. The crash window is: **ClickHouse INSERT succeeded, process dies before CommitMessages.** On restart the same Kafka record is consumed again and a second ClickHouse row with the same `event_id` can appear.

At-least-once is preferred for this MVP over risking log loss: a duplicate error line is cheaper than dropping a payment failure. MergeTree has no unique constraint. Query with `LIMIT 1 BY event_id` when uniqueness matters. The ORDER BY includes `event_id` so a later `ReplacingMergeTree(ingested_at)` can collapse exact-key duplicates without a schema redesign.

Do not treat HTTP 202 alone as proof of ClickHouse persistence.

## Query API (`:8082`)

Read-only ClickHouse access. Protected routes require `Authorization: Bearer <jwt>`. Results are limited to projects the caller can access. All filter values are bound as query parameters. Sort and interval values are allow-listed.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/v1/logs` | Search / filter / paginate |
| GET | `/api/v1/logs/{eventID}` | Single event |
| GET | `/api/v1/stats/overview` | Totals and error rate |
| GET | `/api/v1/stats/timeseries` | Counts over time |
| GET | `/api/v1/stats/services` | Per-service error stats |
| GET | `/api/v1/stats/errors` | Frequent ERROR messages |
| POST | `/api/v1/stream/ticket` | Short-lived one-time WebSocket ticket |
| GET | `/api/v1/stream` | Authenticated WebSocket (`ticket` or Bearer + `project_id`) |
| GET | `/healthz` `/readyz` `/metrics` | Process, ClickHouse ping, Prometheus |

**List filters:** `service`, `level`, `start`, `end` (RFC3339), `q` (substring in message), `event_id`. Combinable.

**Pagination:** keyset on `(timestamp, event_id)`. Default order is **newest first**. Pass `cursor` from `next_cursor` and the same filters. `order=oldest` is the only other sort. `page_size` default 50, max 200. Opaque cursor is base64url `RFC3339Nano|uuid`. Do not use large OFFSET.

**Stats:** optional `start`/`end`; default window last 24h. Timeseries `interval` is one of `1m`, `5m`, `15m`, `1h`, `1d`. Services `sort` is `error_count` (default), `total`, or `error_rate`.

**ClickHouse strategy:** predicates on `service`, `level`, and `timestamp` align with `ORDER BY (service, level, timestamp, event_id)`. Message search uses parameterized `positionCaseInsensitive`. Queries run with `QUERY_TIMEOUT` (default 5s). Get-by-id uses `ORDER BY ingested_at DESC LIMIT 1` if duplicates exist.

Example:

```powershell
curl.exe -sS -H "Authorization: Bearer <jwt>" "http://127.0.0.1:8082/api/v1/logs?service=payment-service&level=ERROR&page_size=20"
```

Example list response:

```json
{
  "logs": [
    {
      "event_id": "11111111-1111-4111-8111-111111111111",
      "timestamp": "2026-08-29T09:00:00Z",
      "ingested_at": "2026-08-29T09:00:01Z",
      "service": "payment-service",
      "level": "ERROR",
      "message": "Payment authorization failed"
    }
  ],
  "page_size": 20,
  "has_more": false
}
```

Metrics include HTTP count/status/duration, in-flight requests, ClickHouse query duration/errors, auth success/failure, API-key rejections, rate-limit rejections, authorization denials, and realtime/WebSocket counters. Labels are coarse (`kind`, `reason`, `scope`) — not user IDs, project IDs, or keys.

## Realtime (WebSocket + Redis)

**Delivery point:** after ClickHouse insert succeeds. The UI does not announce a log as live until it has been written to the durable store. Redis fanout can lag or drop; Refresh/polling still read ClickHouse.

**Channel namespace:** `pulselog:logs:{project-uuid}`. Never API keys, JWTs, or emails. Query-api `PSUBSCRIBE`s `pulselog:logs:*` and delivers only to hub clients subscribed to that `project_id`.

**Connection flow**

1. Dashboard `POST /api/auth/stream-ticket` (cookie) → query-api `POST /api/v1/stream/ticket` (Bearer).
2. Query-api stores a random ticket in Redis for 45s (one-time `GETDEL`).
3. Browser opens `ws://127.0.0.1:8082/api/v1/stream?ticket=...&project_id=...`.
4. Server redeems the ticket, checks the JWT denylist, and verifies `HasProject(project_id)` before upgrade.
5. First frame is `{ "v": 1, "type": "hello", "project_id": "..." }`.
6. Subsequent frames are `log.created` payloads.

Bearer `Authorization` also works for non-browser clients. Do not put the long-lived JWT on the WebSocket URL.

**Project isolation:** the browser-supplied `project_id` is never trusted. Unauthorized projects return `403` before upgrade. Hub maps are keyed by project UUID.

**Message schema**

```json
{
  "v": 1,
  "type": "log.created",
  "data": {
    "event_id": "11111111-1111-4111-8111-111111111111",
    "project_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    "service": "payment-service",
    "level": "ERROR",
    "message": "card declined",
    "timestamp": "2026-08-30T04:00:00.000Z",
    "host": "ip-10-0-1-23",
    "trace_id": "abc123",
    "metadata": { "requestId": "req-1" }
  }
}
```

Payloads are capped at 16 KiB (message truncated, metadata dropped if needed). Server-only fields are not included.

**Reconnect:** the dashboard retries with exponential backoff from 1s to 15s and shows Live / Reconnecting / Disconnected. There is no tight reconnect loop.

**Pause / resume:** Pause live stream keeps counting incoming events (`N new logs`) without rendering them. Resume merges the buffer and de-duplicates by `event_id`.

**Fallback:** Refresh and Off/10s/30s/60s polling remain. The dashboard stays usable if WebSockets fail.

**Degraded mode:** query-api `/readyz` still requires Redis PING (JWT denylist + rate limits). A dead pub/sub subscription is retried in-process and does not take REST offline. Processor `/readyz` does not require Redis.

**Frontend live strategy:** explorer filters match the REST semantics (service, level, substring `q`, event_id). Overview increments total / error / warn immediately, then debounces a REST refresh (~1.5s) for charts, services, and frequent errors so the client does not invent analytics. The time-series chart updates the current rolling window only.

**Metrics:** `pulselog_realtime_published_total`, `pulselog_realtime_publish_errors_total`, `pulselog_realtime_subscribe_errors_total`, `pulselog_ws_connections`, `pulselog_ws_connects_total`, `pulselog_ws_auth_failures_total`, `pulselog_ws_messages_delivered_total`, `pulselog_ws_disconnects_total`.

## Authentication and authorization

Human users use **JWTs** (HS256). Tokens are issued on register/login, sent as `Authorization: Bearer`, and expire after `JWT_TTL` (default 24h). Logout writes the token `jti` to a Redis denylist until expiry. JWTs are used instead of server-side sessions so multiple query-api replicas can validate tokens without sticky sessions.

Passwords are hashed with **argon2id** (`$argon2id$...`). Plaintext passwords are never stored or logged.

Applications ingest with an **API key**:

```
X-API-Key: pl_live_<64 hex chars>
```

`POST /v1/logs`, `POST /v1/logs/batch`, and `POST /ingest` all require a key. The raw secret is shown **once** at creation. PostgreSQL stores only SHA-256(`raw`) plus a display `prefix`. A revoked or unknown key returns 401. The key’s project and service overwrite client fields; a mismatched `service` or `project_id` returns 403.

### Tenancy

```
Organization → Project → Service → Logs (ClickHouse project_id)
```

Users join an organization with a role. All projects in that org are in scope. Ownership lives in PostgreSQL, not ClickHouse. The query API always adds `project_id IN (...)` from the authenticated principal. Client-supplied `project_id` must be a subset of that scope.

### RBAC

Permissions are defined only in `internal/auth`.

| Role | Permissions |
| --- | --- |
| Owner | org, members, projects, services, API keys, query |
| Admin | services, API keys, query |
| Member | query |
| Viewer | query |

### Auth API

| Method | Path |
| --- | --- |
| POST | `/api/v1/auth/register` `{email,password,organization}` (when `AUTH_SIGNUPS=true`) |
| POST | `/api/v1/auth/login` |
| POST | `/api/v1/auth/logout` |
| GET | `/api/v1/auth/me` |
| GET/POST | `/api/v1/orgs/{id}/projects` |
| GET/POST/PATCH/DELETE | `/api/v1/orgs/{id}/members` |
| GET/POST | `/api/v1/projects/{id}/services` |
| GET/POST | `/api/v1/projects/{id}/api-keys` |
| DELETE | `/api/v1/api-keys/{id}` |

Register creates a user, organization (owner), and a `default` project. Then create a service and an API key.

PostgreSQL migrations in `internal/postgres/migrations` run automatically when ingestion-api or query-api starts.

### Rate limiting

Redis fixed-window counters (`rl:{scope}:{id}:{window}` with TTL). Defaults: login 10/min per IP, ingest 120/min per key, query 120/min per IP. Exceeding the limit returns **429**.

Security headers: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Cache-Control: no-store`. CORS is an explicit origin allowlist (`CORS_ORIGINS`); `*` is rejected.

## Dashboard (Phase 5)

Frontend stack: Next.js 15 App Router, React 19, TypeScript (strict), Tailwind CSS, Recharts.

### Routes

| Route | Purpose |
| --- | --- |
| `/login` `/signup` | JWT session via Query API; token stays in an httpOnly cookie |
| `/` | Overview: totals, error rate, volume + error charts, service activity, frequent errors |
| `/logs` | Log explorer: Query API filters, substring search, keyset “Load more”, event detail |
| `/services` | Registered services + activity table |
| `/projects` | List / create (owner) and switch project context |
| `/api-keys` | List / create / revoke (owner, admin). Raw secret shown once |
| `/settings` | Account and tenancy context |

### Authentication flow

1. Sign in or register through the dashboard forms.
2. Next.js `POST /api/auth/login` or `/api/auth/register` forwards to query-api and sets `pulselog_token` (httpOnly, SameSite=Lax, Secure when `COOKIE_SECURE=true` or HTTPS production).
3. Middleware sends anonymous users to `/login`.
4. Page data loads through `/api/backend/...` → `/api/v1/...` with the cookie as `Authorization: Bearer`.
5. Logout clears the cookie and denylists the JWT on query-api.

### API integration

Typed client in `apps/dashboard/src/lib/api.ts`. Components do not call query-api or scatter raw `fetch` for backend resources. Filters, search, and pagination are Query API parameters (`service`, `level`, `start`, `end`, `q`, `event_id`, `cursor`). Message search is a substring, not ranked full-text.

Timeseries by level uses four filtered `/stats/timeseries` calls (ERROR, WARN, INFO, DEBUG) plus an unfiltered series. Missing buckets are not invented.

### Live mode and polling

Overview, logs, and services have a **LIVE** toggle, connection state (Live / Reconnecting / Disconnected), and Pause/Resume. Filters are applied to incoming events; `event_id` suppresses duplicates against REST refresh. UI updates are batched (~200ms) and the explorer caps live-visible rows at 200.

**Refresh** plus optional poll (Off / 10s / 30s / 60s) stay available when live is off or the socket is down.

### Frontend environment

| Variable | Default | Notes |
| --- | --- | --- |
| `QUERY_API_URL` | `http://127.0.0.1:8082` | Server-only. Copy `apps/dashboard/.env.example` to `.env.local` |
| `QUERY_WS_PUBLIC_URL` | `ws://127.0.0.1:8082/api/v1/stream` | Runtime URL from `GET /api/runtime`. JWT is never placed on this URL |
| `NEXT_PUBLIC_QUERY_WS_URL` | same | Local `npm run dev` fallback only |
| `COOKIE_SECURE` | `false` locally | `true` behind HTTPS |

### Screenshots

Add portfolio captures under `docs/screenshots/` (not committed as generated binaries unless you choose to):

- `docs/screenshots/overview.png` — totals, charts, service table
- `docs/screenshots/logs.png` — explorer with filters and a selected event
- `docs/screenshots/api-keys.png` — one-time secret warning after create

See [docs/screenshots/README.md](docs/screenshots/README.md).

## Local startup

```powershell
cp .env.example .env
docker compose -f infrastructure/docker-compose.yml up -d

# terminal 1
go run ./services/ingestion-api

# terminal 2
go run ./services/log-processor

# terminal 3 — allow the dashboard origin for browser WebSockets
$env:CORS_ORIGINS="http://127.0.0.1:3000,http://localhost:3000"
go run ./services/query-api

# terminal 4
cd apps/dashboard
copy .env.example .env.local
npm install
npm run dev
```

After the stack is healthy, seed demo events:

```powershell
.\scripts\seed-dashboard.ps1
```

Then sign in at http://127.0.0.1:3000/login with `dashboard.demo@example.com` / `dashboard-demo-pass` and search for `ord-phase5`.

Health:

- Ingest: http://127.0.0.1:8080/healthz `/readyz` `/metrics`
- Processor: http://127.0.0.1:8081/healthz `/readyz` `/metrics`
- Query API: http://127.0.0.1:8082/healthz `/readyz` `/metrics`
- Dashboard: http://127.0.0.1:3000/login

Ingest (path is `/v1/logs`, not `/ingest`):

```powershell
curl.exe -sS -X POST http://127.0.0.1:8080/v1/logs -H "Content-Type: application/json" --data-binary "@event.json"
```

Verify in ClickHouse:

```powershell
docker exec pulselog-clickhouse-1 clickhouse-client --user pulselog --password pulselog_dev_only --query "SELECT event_id, service, level, message FROM pulselog.logs ORDER BY ingested_at DESC LIMIT 5"
```

Stop:

```powershell
# Ctrl+C the three Go processes
docker compose -f infrastructure/docker-compose.yml down
```

Optional containerized apps (local profile): `docker compose -f infrastructure/docker-compose.yml --profile app up -d --build`

### Production-like Compose (built images, no `go run` / `npm run dev`)

```powershell
copy .env.prod.example .env.prod
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml up -d --build
```

This starts migrate → ingestion-api → processor → query-api → dashboard. Only `:3000`, `:8080`, and `:8082` are published. PostgreSQL, Redis, Kafka, and ClickHouse stay on the internal network. See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md).

## Containers, CI/CD, and Kubernetes

- Production Dockerfiles for all four application services plus `cmd/migrate` (non-root, multi-stage / standalone)
- CI (`.github/workflows/ci.yml`): Go fmt/vet/test, dashboard lint/typecheck/test/build, image builds, Compose + Kubernetes validate, gitleaks (enforced), advisory vuln scans
- Publish (`.github/workflows/publish.yml`): GHCR tags = commit SHA, `main`, and `vMAJOR.MINOR.PATCH` after CI
- Kubernetes app manifests: `deploy/kubernetes/` (managed data services recommended)
- Reverse proxy example: `infrastructure/caddy/Caddyfile`

## Tests

```bash
go test ./internal/... ./services/...
cd apps/dashboard && npm test
```

Processor tests use in-memory ClickHouse and DLQ stubs. Query API tests use a mocked store. Neither requires Kafka or ClickHouse.

## Make targets

| Target | Action |
| --- | --- |
| `make up` | Start Compose infra |
| `make down` | Stop Compose stack |
| `make test` | `go test ./internal/... ./services/...` |
| `make run-ingest` | Run ingestion-api |
| `make run-processor` | Run log-processor |
| `make run-query` | Run query-api |
| `make run-dashboard` | Run Next.js dashboard on :3000 |
| `make fmt` | `gofmt -w` |
| `make up-prod` | Production-like Compose from `.env.prod` |
| `make images` | Build the four app images + migrate |
| `make up-obs` | Prometheus `:9090` + Grafana `:3001` |
| `make load-setup` | Create bench credentials (`tests/load/.credentials.json`) |
| `make load-test` | k6 ingest at 100 events/s for 20s |

## Ingest contract

```json
{
  "service": "payment-service",
  "level": "ERROR",
  "message": "Payment authorization failed",
  "timestamp": "2026-08-29T09:00:00.000Z",
  "host": "ip-10-0-1-23",
  "traceId": "abc123",
  "metadata": {
    "requestId": "req-1",
    "userId": "u-42"
  }
}
```

Send `X-API-Key: pl_live_...`. Successful ingest returns `202 Accepted` with `event_id` values. The handler stamps `project_id` and `service` from the key. ClickHouse is written by the processor.

## Performance (local development)

These numbers were measured on a Windows laptop (i5-11400H, 16 GB RAM, Docker Desktop ~7.6 GiB) and **must not be read as production capacity**. Full tables, failure tests, and reproduce commands: [docs/PERFORMANCE.md](docs/PERFORMANCE.md).

| Measurement | Result on this machine |
| --- | --- |
| Ingest 500 events/s | 9990/9990 HTTP 202; p50 12.5 ms; p95 15.6 ms; p99 21.8 ms |
| Ingest ~1000 events/s | 19714/19714 HTTP 202; ~982/s; p50 14.1 ms; p95 116 ms |
| Local saturation region | ~1000–1200 ingest/s (latency / k6 VUs), not a cluster rating |
| Query API ~105k rows, 8 VUs | 0% errors; HTTP p50 44 ms; p95 75 ms; p99 93 ms |
| Processor vs Kafka | consumed = written in the baseline window; lag 0 after runs |

```powershell
.\scripts\load\setup.ps1
.\scripts\load\ingest.ps1 -Rate 100 -Duration 20s
.\scripts\load\query.ps1
```

## MVP vs later

**Done:** ingest → Kafka → processor → ClickHouse → query API → Next.js dashboard, with JWT users, API keys, RBAC, Redis rate limits, authenticated live updates, and a measured local benchmark report.

**Next:** paid/cloud provisioning is **not** started. Approve Phase 8 before creating AWS/GCP/Azure resources.

**Later:** HPA, OpenTelemetry traces, multi-broker Kafka operations beyond the documented model.

## Current limitations

- Open registration when `AUTH_SIGNUPS=true` (local default). Production Compose and Kubernetes default to `false`
- No email verification, MFA, or password-reset flow
- JWT HMAC secret is a local placeholder unless you set `JWT_SECRET`
- Single-broker Compose Kafka (RF=1) is not a production cluster
- Compose volumes are not a backup/HA story; use managed stores + snapshots
- TLS is terminated at a reverse proxy / load balancer, not inside each service
- Kubernetes manifests do not self-host Kafka, ClickHouse, PostgreSQL, or Redis
- Org-level roles apply to every project in the org (no per-project ACL)
- Pre-Phase-4 ClickHouse rows have a zero `project_id` and are invisible to tenant queries
- Live updates are best-effort after ClickHouse write; Redis/WS loss does not rewind REST
- Live explorer applies filters in the browser using the same rules as the Query API; it is not a second query engine
- Org member invite UI is not in the dashboard (Query API only)
- Message search is substring, not ranked full-text
- Common errors group by exact `message` text
- At-least-once delivery can duplicate `event_id` rows
- Rate limits are fixed-window, not token-bucket
- Local benchmark numbers are not production capacity; see [docs/PERFORMANCE.md](docs/PERFORMANCE.md)
