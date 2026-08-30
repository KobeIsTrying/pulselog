# PulseLog
## Complete User, Developer, and Deployment Manual

This manual describes the PulseLog repository as implemented. It is written so someone who has never seen the project can clone it, run it, send logs, use the dashboard, test it, and deploy it without asking the original author.

Related documents (not replaced by this manual):

- [README.md](../README.md) — project overview and local startup
- [docs/architecture.md](architecture.md) — design notes
- [docs/environment.md](environment.md) — environment contract
- [docs/phases.md](phases.md) — phased delivery history
- [docs/PERFORMANCE.md](PERFORMANCE.md) — local benchmark report
- [docs/DEPLOYMENT.md](DEPLOYMENT.md) — production-style deployment

> **NOTE:** Commands in this manual are for a Windows development machine using PowerShell unless labeled otherwise. Docker commands are the same in PowerShell and Bash. Browser URLs are listed as URLs, not shell commands.

> **WARNING:** Do not copy secrets from examples into a public or production environment. Placeholders such as `pulselog_dev_only` and `pulselog_dev_jwt_only` are local-development values only.

---

## Table of contents

**Part I — Introduction**

1. [Introduction](#1-introduction)
2. [Project purpose](#2-project-purpose)
3. [Project features](#3-project-features)

**Part II — Architecture**

4. [System architecture](#4-system-architecture)
5. [Component explanation](#5-component-explanation)
6. [Data flow](#6-data-flow)

**Part III — Technology stack**

7. [Technologies used](#7-technologies-used)

**Part IV — Repository guide**

8. [Repository structure](#8-repository-structure)

**Part V — Requirements**

9. [System requirements](#9-system-requirements)
10. [Windows + WSL + Docker setup](#10-windows--wsl--docker-setup)

**Part VI — Installation**

11. [Clone the repository](#11-clone-the-repository)
12. [Environment configuration](#12-environment-configuration)
13. [Quick start](#13-quick-start)

**Part VII — Running PulseLog**

14. [Running with production-style Docker Compose](#14-running-with-production-style-docker-compose)
15. [Running in development mode](#15-running-in-development-mode)
16. [Starting PulseLog after restarting the computer](#16-starting-pulselog-after-restarting-the-computer)
17. [Stopping PulseLog](#17-stopping-pulselog)
18. [Restarting PulseLog](#18-restarting-pulselog)
19. [Checking system health](#19-checking-system-health)

**Part VIII — Using the application**

20. [Opening the dashboard](#20-opening-the-dashboard)
21. [Dashboard overview](#21-dashboard-overview)
22. [Log explorer](#22-log-explorer)
23. [Log details](#23-log-details)
24. [Realtime LIVE mode](#24-realtime-live-mode)
25. [Projects](#25-projects)
26. [Services](#26-services)
27. [API keys](#27-api-keys)
28. [Roles and permissions](#28-roles-and-permissions)

**Part IX — Sending logs**

29. [Ingestion API](#29-ingestion-api)
30. [Send your first log](#30-send-your-first-log)
31. [Log schema](#31-log-schema)
32. [Log levels](#32-log-levels)
33. [Integrating an application](#33-integrating-an-application)

**Part X — API reference**

34. [API overview](#34-api-overview)
35. [Authentication flow](#35-authentication-flow)
36. [WebSocket protocol](#36-websocket-protocol)

**Part XI — Monitoring**

37. [Prometheus](#37-prometheus)
38. [Grafana](#38-grafana)
39. [Metrics](#39-metrics)

**Part XII — Testing**

40. [Backend tests](#40-backend-tests)
41. [Dashboard tests](#41-dashboard-tests)
42. [Full regression testing](#42-full-regression-testing)

**Part XIII — Performance testing**

43. [Performance testing](#43-performance-testing)
44. [Running a benchmark](#44-running-a-benchmark)
45. [Existing performance results](#45-existing-performance-results)

**Part XIV — Docker**

46. [Understanding Docker in PulseLog](#46-understanding-docker-in-pulselog)
47. [PulseLog containers](#47-pulselog-containers)
48. [Docker volumes](#48-docker-volumes)
49. [Useful Docker commands](#49-useful-docker-commands)

**Part XV — Troubleshooting**

50. [Troubleshooting guide](#50-troubleshooting-guide)
51. [WSL / Docker recovery](#51-wsl--docker-recovery)
52. [Reading logs](#52-reading-logs)

**Part XVI — Security**

53. [Security architecture](#53-security-architecture)
54. [Production security checklist](#54-production-security-checklist)

**Part XVII — CI/CD**

55. [GitHub Actions](#55-github-actions)
56. [Container registry](#56-container-registry)

**Part XVIII — Deployment**

57. [Production-style deployment](#57-production-style-deployment)
58. [Kubernetes](#58-kubernetes)
59. [Portfolio deployment](#59-portfolio-deployment)
60. [Scaling](#60-scaling)

**Part XIX — Data management**

61. [Data storage](#61-data-storage)
62. [Retention](#62-retention)
63. [Backup](#63-backup)

**Part XX — Maintenance**

64. [Routine maintenance](#64-routine-maintenance)
65. [Updating PulseLog](#65-updating-pulselog)
66. [Rebuilding containers](#66-rebuilding-containers)

**Part XXI — Developer guide**

67. [Making code changes](#67-making-code-changes)
68. [Adding a backend feature](#68-adding-a-backend-feature)
69. [Adding dashboard features](#69-adding-dashboard-features)
70. [Adding database changes](#70-adding-database-changes)

**Part XXII — Performance and design decisions**

71. [Important engineering decisions](#71-important-engineering-decisions)
72. [Delivery guarantees](#72-delivery-guarantees)

**Part XXIII — Limitations**

73. [Current limitations](#73-current-limitations)

**Part XXIV — FAQ**

74. [Frequently asked questions](#74-frequently-asked-questions)

**Part XXV — Command cheat sheet**

75. [Command reference](#75-command-reference)

**Part XXVI — Glossary**

76. [Glossary](#76-glossary)

**Part XXVII — Project status**

77. [Current project status](#77-current-project-status)

---

# 1. Introduction

## What is PulseLog?

PulseLog is a high-throughput distributed log aggregator and metrics dashboard. Applications send structured JSON logs over HTTP. PulseLog validates them, buffers them in Apache Kafka, writes them in batches to ClickHouse, and lets authorized users search, chart, and watch those logs in a Next.js dashboard.

In one sentence: **PulseLog is a self-hosted pipeline that takes logs from many services and makes them searchable, chartable, and viewable in near real time.**

## What problem does it solve?

Modern applications are split into several services. Each service writes logs to its own stdout, file, or cloud sink. When a payment fails, you often need to look in three places, on three machines, with three formats.

PulseLog solves that by providing:

1. A single HTTP ingest endpoint that every service can call.
2. A durable pipeline so a burst of logs does not hit the database at request time.
3. A column store designed for time-range filters and aggregations.
4. A dashboard where a human can find errors, watch live traffic, and manage API keys.

## Why would someone use it?

Use PulseLog when you want to:

- See errors from several backends in one place.
- Search recent logs by service, level, message substring, or event ID.
- Watch a project’s logs appear as they are written (LIVE mode).
- Keep identity (users, projects, API keys) separate from the high-volume log archive.
- Demonstrate a complete ingest → queue → process → query → UI system, including tests, benchmarks, containers, CI, and Kubernetes manifests.

## Who is it designed for?

- **Developers** running local or portfolio/demo deployments.
- **Operators** who need health endpoints, Prometheus metrics, and Docker/Kubernetes packaging.
- **Application services** that can make an HTTP POST with an API key.

It is **not** a hosted SaaS product in this repository. You run the software yourself.

## What types of applications could send logs to it?

Anything that can make an HTTPS (or local HTTP) POST with JSON:

- Backend APIs (payments, auth, inventory, orders)
- Background workers
- Scripts and CLIs
- Load-test clients (the repo includes k6 scenarios)
- Other microservices in the same organization/project

The client does not need a PulseLog SDK. The contract is HTTP + JSON + `X-API-Key`.

## What happens to a log after it is submitted?

A successful ingest returns **HTTP 202 Accepted** and an `event_id`. That does **not** mean ClickHouse has the row yet.

The path is:

1. Ingestion API checks the API key, validates the JSON, stamps `project_id` and `service` from the key.
2. The event is published to Kafka topic `logs-ingest`.
3. The log-processor consumes the topic, batches events, and inserts them into ClickHouse.
4. After a successful insert, the processor publishes a compact payload to Redis for live WebSocket fans.
5. Kafka offsets are committed only after ClickHouse insert (or dead-letter publish) succeeds.
6. The dashboard reads history from the Query API (ClickHouse) and optionally receives live frames over WebSocket.

## Basic terms (plain language)

| Term | Meaning |
| --- | --- |
| **Application logs** | Structured records your code emits: time, level, service, message, optional metadata. |
| **Log ingestion** | Accepting those records over HTTP, validating them, and handing them to the pipeline. PulseLog does this in `ingestion-api`. |
| **Log processing** | Reading the queue, writing durable storage, handling poison messages. PulseLog does this in `log-processor`. |
| **Log storage** | Keeping historical events for search. PulseLog uses ClickHouse table `pulselog.logs`. |
| **Log querying** | Searching and aggregating stored logs over REST. PulseLog does this in `query-api`. |
| **Realtime streaming** | Pushing newly written events to browsers over WebSocket. Best-effort; not the archive. |
| **Monitoring** | Watching the PulseLog system itself (latency, lag, errors) with Prometheus/Grafana. Different from the product dashboard. |

---

# 2. Project purpose

PulseLog exists to give a small team (or a portfolio reviewer) a complete, understandable logging platform: ingest at volume, process asynchronously, store for analytics, query with tenancy, and visualize in a browser.

## Realistic use cases

- **Monitoring backend applications.** Point `payment-service`, `auth-service`, and others at `POST /v1/logs` and watch volume and error rate on Overview.
- **Finding application errors.** Filter the log explorer to `ERROR` / `FATAL`, or use the frequent-errors list (grouped by exact message text).
- **Searching logs.** Filter by time range, service, level, message substring (`q`), or `event_id`.
- **Monitoring multiple services.** Register each service, issue a key bound to that name, and compare error counts on Services.
- **Investigating incidents.** Switch to the affected project, narrow the time window (1h / 6h / 24h / 7d), open the event drawer for host, trace ID, and metadata.
- **Observing error spikes.** Overview charts error trend; LIVE mode bumps totals immediately, then refreshes charts from REST.
- **Viewing logs in realtime.** Enable LIVE on Overview, Logs, or Services. Frames arrive after ClickHouse write.
- **Centralized logging for microservices.** One organization, multiple projects, per-service API keys, ClickHouse `project_id` isolation.

## What PulseLog is not

Based on the implemented product and [docs/DEPLOYMENT.md](DEPLOYMENT.md):

- **Not a full Datadog / Splunk / Elastic Cloud replacement.** There is no APM, no log-based alerting product, no host metrics agent, and no ranked full-text search engine.
- **Not an enterprise SIEM.** There is no threat-intel pipeline, no compliance pack, and org roles apply to every project in the org (no per-project ACL).
- **Not exactly-once delivery.** The pipeline is **at-least-once**. A crash between ClickHouse INSERT and Kafka commit can duplicate an `event_id`.
- **Not a hosted SaaS product.** This repository does not provision paid AWS/GCP/Azure resources.
- **Not a high-availability production cluster out of the box.** Local and production-style Compose use a single Kafka broker (replication factor 1). Kubernetes manifests cover **application** workloads; data stores are expected to be managed services for serious production.
- **Not a guarantee that LIVE equals history.** Realtime is best-effort. ClickHouse via REST is the source of truth.

---

# 3. Project features

### Log ingestion

**What:** HTTP API on port **8080** that accepts one event (`POST /v1/logs` or `POST /ingest`) or a batch (`POST /v1/logs/batch`, max 500 events). Body limit default **5 MiB** (`HTTP_MAX_BODY_BYTES`).

**Why:** Keep ingest cheap and independent of ClickHouse. The request path only validates and publishes to Kafka.

**How you use it:** Send `Content-Type: application/json` and `X-API-Key: pl_live_...`. Success is `202` with `event_ids`.

### Asynchronous processing

**What:** `log-processor` consumes Kafka group `log-processor`, flushes at **100 events** or **500 ms**, retries ClickHouse up to **5** times, then dead-letters.

**Why:** A slow or down database must not fail the client’s ingest HTTP call. Kafka absorbs bursts.

**How you use it:** You do not call the processor. It runs as a worker. Admin health is on **:8081** (not published in production Compose).

### Log storage

**What:** ClickHouse `MergeTree` table `pulselog.logs`, partitioned by day, ordered by `(service, level, timestamp, event_id)`, TTL default **90 days**. Materialized view `logs_per_minute` (SummingMergeTree, TTL default **180 days**) feeds cheap charts.

**Why:** Analytical queries (counts, time series, filters) are a poor fit for PostgreSQL at log volume.

**How you use it:** Indirectly, through the dashboard or Query API. Direct `clickhouse-client` is for debugging.

### Search and querying

**What:** Query API on **:8082**. List logs with keyset pagination; get-by-id; overview; timeseries; per-service stats; frequent errors. All values bound as query parameters; sort/interval allow-listed.

**Why:** The dashboard must not talk to ClickHouse. Authorization (`project_id IN (...)`) is applied in the API.

**How you use it:** Browser → Next.js `/api/backend/*` → Query API with the JWT from an httpOnly cookie. Tools can call Query API with `Authorization: Bearer`.

### Dashboard

**What:** Next.js 15 App Router UI on **:3000**. Overview, log explorer, services, projects, API keys, settings, login/signup. Recharts for volume and error trends.

**Why:** Humans need a BFF so the JWT never sits on a WebSocket URL or in `localStorage`.

**How you use it:** Open `http://127.0.0.1:3000/login` (or your public HTTPS host).

### Authentication

**What:** Human users register/login on Query API. Passwords are **argon2id**. Tokens are **JWT HS256** with TTL default **24h**. Logout writes `jti` to Redis denylist `jwt:deny:{jti}`.

**Why:** Multiple query-api replicas can validate JWTs without sticky sessions.

**How you use it:** Dashboard forms. Cookie name `pulselog_token` (httpOnly, SameSite=Lax, Secure when configured).

### Organizations

**What:** Signup creates an organization and makes the user **owner**. Users belong to orgs via `memberships`.

**Why:** Tenancy starts at the org, then projects.

**How you use it:** Organization name is entered at signup. Member invite UI is **not** in the dashboard; use Query API member routes.

### Projects

**What:** Isolation unit for logs. Register creates a project named `default`. Owners can create more. Dashboard project selector switches context.

**Why:** ClickHouse rows are stamped with `project_id` from the API key. Queries always filter to projects the JWT principal may access.

**How you use it:** Sidebar project dropdown, or Projects page.

### Services

**What:** Named emitters inside a project (`payment-service`). Unique per `(project_id, name)`. API keys are bound to one service name.

**Why:** A key for `payment-service` must not ingest as `auth-service` (that is **403**).

**How you use it:** Services page (owner/admin create). Catalog plus activity stats.

### API keys

**What:** Secrets `pl_live_` + 64 hex characters. PostgreSQL stores SHA-256 of the raw key plus a display `prefix`. Raw secret shown **once** at creation.

**Why:** Applications authenticate without a user JWT. Stolen keys can be revoked.

**How you use it:** API Keys page (owner/admin). Header `X-API-Key` (or Bearer if the token looks like an API key).

### RBAC

**What:** Roles **owner**, **admin**, **member**, **viewer** defined in `internal/auth/rbac.go`. Permissions: org/members/projects/services/API keys/query.

**Why:** Viewers can read logs without rotating keys.

**How you use it:** Role is org-wide (every project in the org). See [§28](#28-roles-and-permissions).

### Rate limiting

**What:** Redis fixed-window counters `rl:{scope}:{id}:{window}`. Defaults: login **10/min** per IP, ingest **120/min** per API key, query **120/min** per IP. HTTP **429** when exceeded. Redis `INCR` errors **fail open** (request allowed).

**Why:** Protect login and ingest from abuse without a token-bucket implementation.

**How you use it:** Invisible until you hit 429. Benchmarks temporarily raise limits in the **process environment only**.

### Realtime streaming

**What:** After ClickHouse insert, processor publishes to Redis `pulselog:logs:{project_id}`. Query-api `PSUBSCRIBE`s `pulselog:logs:*` and fans out to a per-project WebSocket hub. Tickets expire in **45s** (one-time `GETDEL`). Hub buffer default **256**; full buffers drop frames.

**Why:** Operators want new errors on screen without waiting for poll.

**How you use it:** LIVE toggle. Pause/Resume. Status Live / Reconnecting / Disconnected.

### Monitoring

**What:** `/metrics` on ingest, processor, and query-api. Optional Compose profile `obs`: Prometheus and Grafana (“PulseLog operations”).

**Why:** Separate **product** dashboard from **ops** dashboards.

**How you use it:** `make up-obs` (dev) or production Compose `--profile obs`.

### Performance testing

**What:** k6 scripts in `tests/load/` wrapped by `scripts/load/*.ps1`.

**Why:** Phase 7 measured a local baseline. See [docs/PERFORMANCE.md](PERFORMANCE.md).

**How you use it:** `.\scripts\load\setup.ps1` then ingest/query/mixed/ws/rate-limit scripts.

### Containerization

**What:** Production Dockerfiles for ingestion-api, log-processor, query-api, dashboard (standalone), and migrate. Non-root users. Local Compose for infra; production-like Compose for the full stack.

**Why:** Same images in CI, GHCR, Compose, and Kubernetes.

**How you use it:** `make images` or `docker compose ... up -d --build`.

### CI/CD

**What:** `.github/workflows/ci.yml` on PR and `main`; `.github/workflows/publish.yml` pushes GHCR tags after CI.

**Why:** Do not publish untested images.

**How you use it:** Push to GitHub. No deploy-to-cloud job is included.

### Kubernetes

**What:** Kustomize app manifests in `deploy/kubernetes/` (namespace, ConfigMap, migrate Job, Deployments, Services, Ingress). Secrets are **not** applied from git.

**Why:** A documented path onto a cluster without pretending data stores are in-cluster HA.

**How you use it:** See [§58](#58-kubernetes) and [docs/DEPLOYMENT.md](DEPLOYMENT.md).

---

# 4. System Architecture

PulseLog is a small distributed system: three Go services, one Next.js BFF, and four data stores (Kafka, ClickHouse, PostgreSQL, Redis).

```mermaid
flowchart TB
  subgraph clients [Clients]
    App["Application + API key"]
    Browser["Browser"]
  end

  subgraph edge [Published edge]
    Ingest["ingestion-api :8080"]
    Query["query-api :8082 REST + WebSocket"]
    Dash["Next.js dashboard :3000"]
  end

  subgraph data [Data plane]
    Kafka["Kafka logs-ingest / logs-dlq"]
    Proc["log-processor :8081"]
    CH["ClickHouse pulselog.logs"]
    PG["PostgreSQL identity"]
    Redis["Redis limits / denylist / pub-sub / tickets"]
  end

  App -->|"POST /v1/logs X-API-Key"| Ingest
  Ingest -->|produce| Kafka
  Kafka -->|consume group log-processor| Proc
  Proc -->|batch INSERT| CH
  Proc -->|DLQ on poison / exhausted writes| Kafka
  Proc -->|PUBLISH after insert| Redis
  Browser --> Dash
  Dash -->|"BFF cookie JWT /api/auth /api/backend"| Query
  Dash -->|"ticket then ws /api/v1/stream"| Query
  Query --> CH
  Query --> PG
  Query --> Redis
  Ingest --> PG
  Ingest --> Redis
```

**PostgreSQL** stores users, organizations, projects, services, memberships, API key hashes, and audit events. It does **not** store log bodies.

**Kafka** is the ingest buffer, not the long-term archive.

**ClickHouse** is the log archive and analytics store.

**Redis** is ephemeral operational state: rate-limit windows, JWT denylist, one-time WebSocket tickets, and live pub/sub. It does not cache dashboard aggregations in the current code.

**Production routing** (from README / DEPLOYMENT):

```
https://pulselog.example.com        → dashboard :3000
https://api.pulselog.example.com    → query-api :8082
https://ingest.pulselog.example.com → ingestion-api :8080
```

TLS terminates at the reverse proxy, load balancer, or Ingress. PostgreSQL, Redis, Kafka, ClickHouse, processor :8081, and `/metrics` stay private.

---

# 5. Component explanation

### Ingestion API

| | |
| --- | --- |
| **Purpose** | Validate events and publish to Kafka. Never writes ClickHouse. |
| **Port** | `HTTP_ADDR` default `:8080` |
| **Auth** | API key required on `POST /v1/logs`, `POST /v1/logs/batch`, `POST /ingest` |
| **Validation** | JSON decode, size limit, `models.LogEvent` rules; key’s `service`/`project_id` overwrite client fields |
| **Kafka** | Topic `logs-ingest`; message key = service name (ordering within a partition); acks = one; Snappy; auto-create **disabled** |
| **Health** | `GET /healthz` process up; `GET /readyz` Kafka + PostgreSQL + Redis; `GET /metrics` |
| **Rate limit** | 120/min per key id (`rl:ingest:key:{id}:{window}`); missing-key attempts limited by IP |

Kafka down returns **503** `kafka_unavailable`. Invalid JSON **400**. Payload too large **413**. Mismatched service/project **403**. Bad/revoked/missing key **401**.

### Kafka

| Topic | Local partitions | Local RF | Purpose |
| --- | --- | --- | --- |
| `logs-ingest` | 6 | 1 | Validated events |
| `logs-dlq` | 3 | 1 | Processor dead letters |

Topics are created by Compose service `kafka-init` (`--if-not-exists`). Consumer group: **`log-processor`**.

**Why Kafka:** Decouple HTTP accept from ClickHouse write; replay after processor downtime; scale consumers with partitions.

Local Compose uses Apache Kafka **3.9.0** in KRaft mode (combined broker+controller), **one node**. That is not a production cluster.

### Log processor

- Fetches from `logs-ingest`.
- Flush: `PROCESSOR_BATCH_SIZE=100` or `PROCESSOR_BATCH_TIMEOUT=500ms`.
- Parse/validate; `invalid_json` / `validation_failed` go to DLQ immediately.
- In-batch de-duplication by `event_id` only.
- ClickHouse insert with **5** attempts, backoff from **200ms**, cap **5s**.
- Exhausted writes → DLQ reason `clickhouse_write_failed`.
- After successful insert: Redis publish per event with a real `project_id`. Redis failure is logged/counted and **does not** block commit.
- Commit Kafka offsets only after insert **or** successful DLQ publish. If DLQ publish fails, offsets are **not** committed.
- `/readyz` requires Kafka + ClickHouse. Redis is optional (startup warns if ping fails).
- Graceful shutdown flushes the in-memory batch (`PROCESSOR_SHUTDOWN_GRACE` default 15s).

### ClickHouse

**Why:** Time-partitioned MergeTree, skip indexes, and materialized views fit log analytics better than row-store PostgreSQL.

**Schema:** See README table and `infrastructure/clickhouse/init.sql` / `internal/clickhouse/schema.go` `EnsureSchema`. Image **24.8**. Native port **9000**; HTTP **8123** (published only on local Compose).

**Querying:** Query API uses parameterized predicates. Message search is `positionCaseInsensitive` (substring). Get-by-id uses `ORDER BY ingested_at DESC LIMIT 1` if duplicates exist.

**TTL:** `CLICKHOUSE_TTL_DAYS` (default 90) on raw logs; `CLICKHOUSE_MV_TTL_DAYS` (default 180) on `logs_per_minute`. Applied with `MODIFY TTL` on migrate/processor/query startup so existing volumes pick up the window.

### PostgreSQL

Image **postgres:16-alpine**. Database `pulselog`. Tables from `internal/postgres/migrations/001_identity.sql`:

`users`, `organizations`, `projects`, `services`, `memberships`, `api_keys`, `audit_events`, plus `schema_migrations`.

Migrations run via `cmd/migrate` and again on ingestion-api / query-api start, serialized with `pg_advisory_lock(59204711)`.

### Redis

Image **redis:7-alpine**, AOF enabled in Compose.

| Use | Who |
| --- | --- |
| Rate-limit counters | ingestion-api, query-api |
| JWT denylist `jwt:deny:{jti}` | query-api |
| WebSocket tickets `ws:ticket:{id}` (45s, GETDEL) | query-api |
| Pub/sub `pulselog:logs:{project_id}` | processor publish, query-api subscribe |

Query-api and ingestion-api **readiness fail** if Redis is down. Processor readiness does not require Redis. Historical logs in ClickHouse are unaffected by Redis loss.

### Query API

- JWT on protected REST (`Authorization: Bearer`).
- REST: logs, stats, orgs, projects, services, API keys, members, stream ticket.
- WebSocket `GET /api/v1/stream?ticket=...&project_id=...` or Bearer + `project_id`.
- CORS allow-list (`CORS_ORIGINS`); `*` is ignored, never reflected.
- `/readyz`: ClickHouse + PostgreSQL + Redis.
- Query timeout `QUERY_TIMEOUT` default 5s → HTTP 504.
- Max **256** concurrent WebSocket connections per process (`wsMaxConns`).

### Next.js dashboard

- UI: Overview charts, log table + detail drawer, services, projects, API keys, settings.
- LIVE bar + Refresh bar (time range 1h/6h/24h/7d; poll Off/10s/30s/60s).
- Typed client `apps/dashboard/src/lib/api.ts`. Pages do not call query-api from the browser except WebSocket (ticket in query string, not JWT).
- Production image: `output: "standalone"`, `node server.js`, user **1001**.

---

# 6. Data flow

## Example: payment-service ERROR from HTTP to the dashboard

```
payment-service creates ERROR
        ↓
POST /v1/logs  +  X-API-Key
        ↓
API key verified (SHA-256 lookup, not revoked)
        ↓
service/project stamped from the key  (mismatch → 403)
        ↓
event_id assigned if missing
        ↓
Kafka logs-ingest  (key = service name)
        ↓
HTTP 202 { accepted, event_ids }   ← client returns here
        ↓
log-processor batch
        ↓
ClickHouse INSERT pulselog.logs
        ↓
Redis PUBLISH pulselog:logs:{project_id}
        ↓
query-api hub → WebSocket log.created
        ↓
dashboard LIVE (if connected and filters match)
```

**Stage notes**

1. **Create.** The app builds JSON (`service`, `level`, `message`, optional timestamp/host/traceId/metadata).
2. **POST.** TLS (production) or `http://127.0.0.1:8080` (local).
3. **Key.** Unknown/revoked/missing → 401. Rate limit → 429.
4. **Bind.** Client `service` must be empty or equal the key’s service.
5. **Kafka.** If produce fails → 503. Event is not in ClickHouse.
6. **Processor.** Malformed Kafka values go to `logs-dlq`. Valid rows insert as a batch.
7. **ClickHouse.** Durable history. Duplicate `event_id` possible across the insert/commit crash window.
8. **Redis.** Compact JSON envelope, max 16 KiB (message truncated, metadata dropped if needed). Failure does not roll back ClickHouse or block Kafka commit.
9. **WebSocket.** Only subscribers whose ticket/JWT `HasProject(project_id)` passed. Unauthorized `project_id` is **403 before upgrade**.
10. **UI.** Explorer applies the same filter rules in the browser for live rows; REST remains the historical path.

## Historical REST query path (no Kafka)

```
Browser
  → GET /api/backend/logs?...   (cookie)
  → Next.js BFF attaches Authorization: Bearer
  → query-api GET /api/v1/logs
  → authorize project_id subset of principal
  → ClickHouse parameterized SELECT
  → JSON { logs, page_size, has_more, next_cursor }
```

Refresh and polling always use this path. If WebSocket is down, history is still available.

---

# 7. Technologies used

| Technology | Role in PulseLog | Why it is used |
| --- | --- | --- |
| Go 1.25 | ingestion-api, log-processor, query-api, migrate | Small static binaries, good HTTP/Kafka/CH libraries, `CGO_ENABLED=0` images |
| Next.js 15 (App Router) | Dashboard BFF + UI | Server routes for cookies; `standalone` output for Docker |
| React 19 | Dashboard components | UI for charts, tables, live bar |
| TypeScript (strict) | Dashboard | Typed API client and pages |
| Tailwind CSS 4 | Dashboard styling | Utility CSS in `globals.css` / components |
| Recharts | Volume and error charts | Overview visualizations |
| Vitest + Testing Library | Dashboard unit tests | `npm test` |
| Apache Kafka 3.9.0 | Ingest buffer and DLQ | Decouple HTTP from ClickHouse; consumer groups |
| segmentio/kafka-go | Kafka client | Produce, consume, DLQ |
| ClickHouse 24.8 | Log archive + MV | Column store, TTL, MergeTree |
| clickhouse-go/v2 | ClickHouse driver | Native protocol :9000 |
| PostgreSQL 16 | Identity and configuration | Users, orgs, projects, keys |
| pgx/v5 | PostgreSQL driver | Pool + migrations |
| Redis 7 | Limits, denylist, tickets, pub/sub | Shared state across API replicas |
| go-redis/v9 | Redis client | INCR script, GETDEL, PSUBSCRIBE |
| golang-jwt/jwt/v5 | JWT HS256 | Stateless user auth |
| argon2 (x/crypto) | Password hashing | ` $argon2id$ ` PHC strings |
| gorilla/websocket | Query API WebSockets | Live frames |
| Prometheus client_golang | `/metrics` | RED-style and pipeline counters |
| Prometheus v2.54.1 | Optional scrape | Compose profile `obs` |
| Grafana 11.2.2 | Optional ops UI | Provisioned “PulseLog operations” |
| Docker / Compose | Local and portfolio runtime | Infra + app images |
| Caddy 2.8 | Optional reverse proxy | Compose profile `proxy`; WS upgrades |
| Kubernetes + Kustomize | App deploy manifests | `deploy/kubernetes/` |
| kubeconform | CI manifest validate | Enforced in CI |
| GitHub Actions | CI and GHCR publish | `ci.yml`, `publish.yml` |
| gitleaks | Secret scan | Enforced in CI |
| k6 | Load tests | `tests/load/*.js` |
| PowerShell | Windows scripts | seed, load, e2e |

---

# 8. Repository structure

```
pulselog/
├── apps/dashboard/          Next.js UI, BFF routes, Vitest
├── services/
│   ├── ingestion-api/       Go ingest HTTP + Kafka produce
│   ├── log-processor/       Go Kafka consumer → ClickHouse + Redis
│   └── query-api/           Go REST, admin, WebSocket
├── internal/                Shared Go libraries
├── cmd/migrate/             One-shot Postgres + ClickHouse schema job
├── infrastructure/          Compose, Caddy, Prometheus, Grafana, init SQL
├── deploy/kubernetes/       Kustomize app manifests
├── scripts/                 Seed, load wrappers, e2e
├── tests/load/              k6 scenarios
├── docs/                    Architecture, env, phases, performance, deploy, this manual
├── .github/workflows/       ci.yml, publish.yml
├── Makefile
├── go.mod / go.sum
├── .env.example
└── .env.prod.example
```

### Directories

| Path | What belongs there |
| --- | --- |
| `apps/dashboard/` | Next.js 15 app: `src/app` routes, `src/components`, `src/lib`, Dockerfile |
| `services/` | One Go `main` per process |
| `internal/` | Importable packages: `auth`, `clickhouse`, `config`, `httpx`, `identity`, `kafka`, `logger`, `metrics`, `models`, `postgres`, `ratelimit`, `realtime` |
| `infrastructure/` | `docker-compose.yml` (dev infra), `docker-compose.prod.yml`, ClickHouse/Postgres init SQL, Caddyfile, Prometheus/Grafana |
| `cmd/migrate/` | Dedicated migration container/job |
| `scripts/` | `seed-dashboard.ps1`, `e2e-phase6.ps1`, `scripts/load/*.ps1` |
| `tests/load/` | k6 JS, gitignored results and `.credentials.json` |
| `deploy/kubernetes/` | Namespace, ConfigMap, Deployments, Services, Ingress, migrate Job, secret **example** |
| `docs/` | Human documentation |
| `.github/workflows/` | CI and publish |

### Important files

| File | Role |
| --- | --- |
| `go.mod` | Module `github.com/pulselog/pulselog`, Go 1.25.0 |
| `Makefile` | `up`, `down`, `test`, `run-*`, `up-prod`, `images`, `up-obs`, load helpers |
| `.env.example` | Local host-run / local Compose contract; copy to `.env` |
| `.env.prod.example` | Production-like Compose; copy to `.env.prod` (gitignored) |
| `infrastructure/docker-compose.yml` | Dev: publishes Kafka/CH/PG/Redis; app services behind `--profile app` |
| `infrastructure/docker-compose.prod.yml` | Built images; only 3000/8080/8082 published; internal `data` network |
| `internal/postgres/migrations/001_identity.sql` | Identity schema |
| `infrastructure/clickhouse/init.sql` | First-volume ClickHouse schema |

---

# 9. System requirements

## Required for Docker-only (production-style Compose)

| Requirement | Notes |
| --- | --- |
| Git | Clone the repo |
| Docker Desktop | Engine + `docker compose` v2 |
| Windows virtualization | Required for Docker Desktop’s Linux VM |
| WSL2 | Typical Docker Desktop backend on Windows |
| Disk | Kafka + ClickHouse images and volumes need several GiB |
| RAM | Local Kafka ~1 GiB, ClickHouse ~0.7–1 GiB; 16 GB host is comfortable |

You do **not** need Go or Node.js if you only run `infrastructure/docker-compose.prod.yml`.

## Required for development (`go run` / `npm run dev`)

| Requirement | Verified from the repo |
| --- | --- |
| Go | **1.25.x** (`go.mod` `go 1.25.0`; CI `setup-go` 1.25.x; images `golang:1.25-alpine`) |
| Node.js | **20** (CI `node-version: "20"`; dashboard image `node:20-alpine`) |
| npm | Comes with Node; dashboard uses `package-lock.json` |
| PowerShell | Seed and load scripts are `.ps1` |
| Docker Compose stack | Kafka, ClickHouse, PostgreSQL, Redis still run in Docker |

## Optional

| Tool | Used for |
| --- | --- |
| k6 | Load tests (`tests/load`); scripts also try `docker run grafana/k6` |
| kubectl | Applying `deploy/kubernetes` |
| kustomize | `kubectl apply -k` or the CI image `kustomize:v5.4.3` |
| curl / curl.exe | Health and ingest examples |

There is no `engines` field in `apps/dashboard/package.json`. Match CI: Node 20.

---

# 10. Windows + WSL + Docker setup

PulseLog’s data plane runs Linux containers. On Windows that means Docker Desktop using a Linux engine (typically **WSL2**).

### 1. Enable virtualization

In BIOS/UEFI, enable Intel VT-x or AMD-V. Windows Features should include **Virtual Machine Platform** (and **Windows Subsystem for Linux** if you use WSL).

### 2. Install WSL2 and a Linux distro

In an elevated PowerShell:

```powershell
wsl --status
wsl --list --verbose
```

Successful `wsl --status` generally shows a default distribution and WSL2 as the default version. `wsl --list --verbose` shows `VERSION` **2** for the distro Docker uses (often Ubuntu).

If WSL is missing:

```powershell
wsl --install
```

Reboot if Windows asks.

### 3. Install Docker Desktop

Install Docker Desktop for Windows. Enable **Use the WSL 2 based engine** and enable integration with your distro under Settings → Resources → WSL integration.

### 4. Confirm Docker

```powershell
docker version
docker info
docker compose version
```

- `docker version` should show a **Server** section (Engine is running). Client-only output means the engine is not up.
- `docker info` should show a Linux OS for the server (not an error about the pipe/named pipe).
- `docker compose version` should report Compose v2.

> **TIP:** Docker Desktop must be **running** (system tray icon) before any `docker compose` command. Closing Cursor does not stop Docker. Restarting Windows does; start Docker Desktop again.

---

# 11. Clone the repository

This working copy has **no git remote configured**. Use your actual URL:

```powershell
git clone <repository-url>
cd pulselog
```

If you already have the files, `cd` into the repo root (the directory that contains `go.mod` and `infrastructure/`).

---

# 12. Environment configuration

| File | Committed? | Purpose |
| --- | --- | --- |
| `.env.example` | Yes | Template for **local** host-run binaries and documentation of defaults |
| `.env` | No (gitignored) | Your local copy of `.env.example` |
| `.env.prod.example` | Yes | Template for production-like Compose |
| `.env.prod` | No (gitignored) | Secrets for `docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml` |
| `apps/dashboard/.env.example` | Yes | Template for `npm run dev` |
| `apps/dashboard/.env.local` | No | Local Next.js env |

Copy:

```powershell
copy .env.example .env
copy .env.prod.example .env.prod
copy apps\dashboard\.env.example apps\dashboard\.env.local
```

### Generating secrets

Replace production placeholders with long random values. Example PowerShell:

```powershell
# 64 hex chars — suitable for JWT_SECRET or similar
-join ((1..64) | ForEach-Object { '{0:x}' -f (Get-Random -Maximum 16) })
```

Use a strong unique password for `POSTGRES_PASSWORD` and `CLICKHOUSE_PASSWORD`. Put the same password into `POSTGRES_DSN`.

### Environment variable catalog

Defaults below match `internal/config/config.go` and the example env files. “Required?” means required for a correct production-like run, not that the process always refuses to start without it.

| Variable | Service | Required? | Default | Purpose | Secret? | Example |
| --- | --- | --- | --- | --- | --- | --- |
| `ENV` | Go services | No | `development` | Runtime label; `production` tightens signup default | No | `production` |
| `LOG_LEVEL` | Go services | No | `info` | `debug` `info` `warn` `error` | No | `info` |
| `HTTP_ADDR` | ingestion-api | No | `:8080` | Listen address | No | `:8080` |
| `HTTP_MAX_BODY_BYTES` | ingestion-api | No | `5242880` | Max ingest body | No | `5242880` |
| `PROCESSOR_HTTP_ADDR` | log-processor | No | `:8081` | Health/metrics listen | No | `:8081` |
| `QUERY_HTTP_ADDR` | query-api | No | `:8082` | REST + WS listen | No | `:8082` |
| `QUERY_TIMEOUT` | query-api | No | `5s` | ClickHouse query timeout | No | `5s` |
| `WS_HUB_BUFFER` | query-api | No | `256` | Per-subscriber live buffer | No | `256` |
| `KAFKA_BROKERS` | ingest, processor | Yes | `localhost:9092` | Broker list; Compose network uses `kafka:19092` | No | `localhost:9092` |
| `KAFKA_TOPIC_INGEST` | ingest, processor | Yes | `logs-ingest` | Ingest topic | No | `logs-ingest` |
| `KAFKA_TOPIC_DLQ` | processor | No | `logs-dlq` | Dead-letter topic | No | `logs-dlq` |
| `KAFKA_CONSUMER_GROUP` | processor | No | `log-processor` | Consumer group | No | `log-processor` |
| `KAFKA_WRITE_TIMEOUT` | ingest | No | `5s` | Produce timeout | No | `5s` |
| `PROCESSOR_BATCH_SIZE` | processor | No | `100` | Flush size | No | `100` |
| `PROCESSOR_BATCH_TIMEOUT` | processor | No | `500ms` | Flush time | No | `500ms` |
| `PROCESSOR_MAX_ATTEMPTS` | processor | No | `5` | ClickHouse retries | No | `5` |
| `PROCESSOR_RETRY_BACKOFF` | processor | No | `200ms` | Backoff start | No | `200ms` |
| `PROCESSOR_SHUTDOWN_GRACE` | processor | No | `15s` | Flush on SIGTERM | No | `15s` |
| `CLICKHOUSE_WRITE_TIMEOUT` | processor | No | `10s` | Insert timeout | No | `10s` |
| `CLICKHOUSE_TABLE` | processor, query, migrate | No | `logs` | Table name | No | `logs` |
| `CLICKHOUSE_ADDR` | processor, query, migrate | Yes | `localhost:9000` | Native protocol | No | `clickhouse:9000` |
| `CLICKHOUSE_DATABASE` | same | No | `pulselog` | Database | No | `pulselog` |
| `CLICKHOUSE_USER` | same | No | `pulselog` | User | No | `pulselog` |
| `CLICKHOUSE_PASSWORD` | same | Prod yes | local placeholder | ClickHouse password | **Yes** | *(set your own)* |
| `CLICKHOUSE_TTL_DAYS` | migrate, processor, query | No | `90` | Raw log TTL | No | `90` |
| `CLICKHOUSE_MV_TTL_DAYS` | same | No | `180` | MV TTL | No | `180` |
| `POSTGRES_DSN` | ingest, query, migrate | Yes | local DSN | Identity database | **Yes** | `postgres://user:pass@host:5432/pulselog?sslmode=disable` |
| `POSTGRES_USER` | Compose | Compose | `pulselog` | Init user | No | `pulselog` |
| `POSTGRES_PASSWORD` | Compose | Prod yes | local placeholder | Init password | **Yes** | *(set your own)* |
| `POSTGRES_DB` | Compose | No | `pulselog` | Database name | No | `pulselog` |
| `REDIS_ADDR` | ingest, query, processor | Yes | `localhost:6379` | Redis host:port | No | `redis:6379` |
| `JWT_SECRET` | query-api | Prod / query-api | local placeholder in non-prod | HMAC secret | **Yes** | long random string |
| `JWT_TTL` | query-api | No | `24h` | Token lifetime | No | `24h` |
| `AUTH_SIGNUPS` | query-api | No | `true` unless `ENV=production` | Allow `POST /api/v1/auth/register` | No | `false` |
| `CORS_ORIGINS` | ingest, query | Prod yes | empty | Allowed browser origins; `*` rejected | No | `http://127.0.0.1:3000,http://localhost:3000` |
| `RATE_LIMIT_LOGIN` | query-api | No | `10` | Login attempts / window | No | `10` |
| `RATE_LIMIT_LOGIN_WINDOW` | query-api | No | `1m` | Login window | No | `1m` |
| `RATE_LIMIT_INGEST` | ingest | No | `120` | Ingest / window / key | No | `120` |
| `RATE_LIMIT_INGEST_WINDOW` | ingest | No | `1m` | Ingest window | No | `1m` |
| `RATE_LIMIT_QUERY` | query-api | No | `120` | Query / WS / window / IP | No | `120` |
| `RATE_LIMIT_QUERY_WINDOW` | query-api | No | `1m` | Query window | No | `1m` |
| `QUERY_API_URL` | dashboard server | No | `http://127.0.0.1:8082` | BFF upstream | No | `http://query-api:8082` |
| `QUERY_WS_PUBLIC_URL` | dashboard | Prod yes | `ws://127.0.0.1:8082/api/v1/stream` | Browser WebSocket URL from `GET /api/runtime` | No | `wss://api.example.com/api/v1/stream` |
| `NEXT_PUBLIC_QUERY_WS_URL` | dashboard local dev | No | same as above | Fallback when `/api/runtime` unused | No | `ws://127.0.0.1:8082/api/v1/stream` |
| `COOKIE_SECURE` | dashboard | HTTPS yes | `false` unless `NODE_ENV=production` | Session cookie Secure flag | No | `true` |
| `GRAFANA_ADMIN_PASSWORD` | Grafana (prod obs) | If enabling obs | must set for prod Compose | Grafana admin | **Yes** | *(set your own)* |

Do **not** put `JWT_SECRET` or raw API keys in `NEXT_PUBLIC_*` variables.

> **WARNING:** Phase 7 used `RATE_LIMIT_INGEST=1000000` only in the **host process environment** during benchmarks. Never put that in `.env.prod` or Kubernetes ConfigMap.

---

# 13. Quick start

Shortest reliable path on Windows: **production-style Compose** (all app services in containers).

```powershell
copy .env.prod.example .env.prod
# Edit .env.prod: JWT_SECRET, POSTGRES_PASSWORD, POSTGRES_DSN, CLICKHOUSE_PASSWORD
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml up -d --build
```

Wait until healthchecks pass:

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml ps
curl.exe -sS http://127.0.0.1:8080/readyz
curl.exe -sS http://127.0.0.1:8082/readyz
```

Open the dashboard: [http://127.0.0.1:3000/login](http://127.0.0.1:3000/login)

Production Compose defaults `AUTH_SIGNUPS=false`. Either set `AUTH_SIGNUPS=true` in `.env.prod` for a private demo, or create the first user out of band. Local **development** Compose/host-run defaults signups **on**.

Equivalent Make target (also uses `.env.prod`):

```powershell
make up-prod
```

---

# 14. Running with production-style Docker Compose

From the **repository root**:

```powershell
docker compose --env-file .env.prod `
  -f infrastructure/docker-compose.prod.yml `
  up -d --build
```

**What this does**

1. Reads secrets from `.env.prod`.
2. Builds migrate, ingestion-api, log-processor, query-api, dashboard images.
3. Starts Kafka (internal only), ClickHouse, PostgreSQL, Redis.
4. Runs `kafka-init` to create topics.
5. Runs `migrate` to apply Postgres migrations and ClickHouse `EnsureSchema`.
6. Starts app containers. Only **:3000**, **:8080**, and **:8082** are published.
7. Processor, Kafka, ClickHouse, PostgreSQL, and Redis sit on network `pulselog_data` (`internal: true`).

Check:

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml ps
```

Expected services (names prefixed with Compose project `pulselog`): `kafka`, `clickhouse`, `postgres`, `redis`, `ingestion-api`, `log-processor`, `query-api`, `dashboard`, plus one-shot `kafka-data-init`, `kafka-init`, `migrate`.

Optional:

```powershell
# TLS reverse proxy (Caddy) — profile proxy
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml --profile proxy up -d --build

# Prometheus + Grafana bound to 127.0.0.1 — profile obs
# Set GRAFANA_ADMIN_PASSWORD in .env.prod first
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml --profile obs up -d
```

From inside `infrastructure/`, the same stack is `docker compose --env-file ../.env.prod -f docker-compose.prod.yml ...` ([infrastructure/README.md](../infrastructure/README.md)). Prefer the repo-root form.

---

# 15. Running in development mode

Development keeps **infra in Docker** and **Go/Node on the host** so you can iterate with `go run` / `npm run dev`.

### 1. Start infrastructure (terminals can close after this)

```powershell
copy .env.example .env
docker compose -f infrastructure/docker-compose.yml up -d
# or: make up
```

This publishes Kafka **9092**, ClickHouse **8123/9000**, PostgreSQL **5432**, Redis **6379**. Application containers are **not** started unless you use `--profile app`.

### 2–5. Start processes (leave these terminals open)

**Terminal 1 — ingestion API**

```powershell
go run ./services/ingestion-api
# or: make run-ingest
```

**Terminal 2 — processor**

```powershell
go run ./services/log-processor
# or: make run-processor
```

**Terminal 3 — query API** (allow the dashboard origin for browser WebSockets)

```powershell
$env:CORS_ORIGINS="http://127.0.0.1:3000,http://localhost:3000"
go run ./services/query-api
# or: make run-query
```

**Terminal 4 — dashboard**

```powershell
cd apps/dashboard
copy .env.example .env.local
npm install
npm run dev
# from repo root: make run-dashboard
```

Host binaries use `localhost` brokers/addresses from `.env.example`. Optional all-in-containers on the **local** Compose file:

```powershell
docker compose -f infrastructure/docker-compose.yml --profile app up -d --build
# or: make up-app
```

That profile **does** publish processor **8081**. After the stack is healthy, seed demo events:

```powershell
.\scripts\seed-dashboard.ps1
```

Then sign in at http://127.0.0.1:3000/login with the seed account (see [§20](#20-opening-the-dashboard)).

---

# 16. Starting PulseLog after restarting the computer

Windows does not keep Linux containers or `go run` processes across a reboot.

### If you use production-style Compose (apps in containers)

1. Start **Docker Desktop**. Wait until the engine is running (`docker info` works).
2. Open a terminal in the repo root.
3. Start the stack (images already built; omit `--build` if you did not change code):

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml up -d
```

4. Verify `docker compose ... ps` and `/readyz`.
5. Open http://127.0.0.1:3000/login

You do **not** start Go or Next.js on the host. Those processes live **inside** containers.

### If you use development mode

1. Start Docker Desktop; wait for the engine.
2. `docker compose -f infrastructure/docker-compose.yml up -d`
3. Re-run the three `go run` terminals and `npm run dev`.

### Containers vs host processes

| Kind | Examples | Survives closing Cursor? | Survives Windows reboot? |
| --- | --- | --- | --- |
| Containers | Kafka, CH, PG, Redis, prod app images | Yes, while Docker Desktop runs | No; start Docker + `compose up` again |
| Go processes | `go run ./services/...` | No | No |
| Next.js | `npm run dev` | No | No |

---

# 17. Stopping PulseLog

### Host processes

In each Go/Next terminal: **Ctrl+C**.

### Compose — pause containers, keep volumes

**Local infra:**

```powershell
docker compose -f infrastructure/docker-compose.yml stop
```

**Production-style:**

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml stop
```

`stop` keeps containers and named volumes.

### Compose — remove containers, keep volumes

```powershell
docker compose -f infrastructure/docker-compose.yml down
# Makefile: make down   (local file only, not prod)
```

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml down
```

`down` removes containers and the default network. **Named volumes remain.** Kafka/ClickHouse/Postgres/Redis data stay.

### Compose — delete volumes (destroys local data)

```powershell
docker compose -f infrastructure/docker-compose.yml down -v
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml down -v
```

> **WARNING:** `docker compose down -v` deletes named volumes (`kafka_data`, `clickhouse_data`, `postgres_data`, `redis_data`, and in prod `caddy_data`). That **erases local logs, users, and API keys**. Use it only when you intentionally want a clean slate.

---

# 18. Restarting PulseLog

### Entire local infra stack

```powershell
docker compose -f infrastructure/docker-compose.yml restart
```

### Entire production-style stack

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml restart
```

### One container

```powershell
docker compose -f infrastructure/docker-compose.yml restart kafka
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml restart query-api
```

Service names are Compose service names (`ingestion-api`, `log-processor`, `query-api`, `dashboard`, `clickhouse`, …).

### One Go service (dev)

Ctrl+C, then `go run ./services/ingestion-api` (or processor / query-api). Environment variables such as `CORS_ORIGINS` or raised rate limits must be set again in that terminal.

### Dashboard (dev)

Ctrl+C in the `npm run dev` terminal, then `npm run dev` again.

### Dashboard (container)

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml restart dashboard
```

---

# 19. Checking system health

| Check | URL / command | Meaning |
| --- | --- | --- |
| Ingest liveness | http://127.0.0.1:8080/healthz | Process is up |
| Ingest readiness | http://127.0.0.1:8080/readyz | Kafka + PostgreSQL + Redis |
| Processor liveness | http://127.0.0.1:8081/healthz | Process up (**dev / local app profile only**; not published in prod Compose) |
| Processor readiness | http://127.0.0.1:8081/readyz | Kafka + ClickHouse |
| Query liveness | http://127.0.0.1:8082/healthz | Process up |
| Query readiness | http://127.0.0.1:8082/readyz | ClickHouse + PostgreSQL + Redis |
| Dashboard | http://127.0.0.1:3000/login | Process up (BFF fails later if query-api is down) |
| Metrics | `.../metrics` on 8080/8081/8082 | Prometheus text |

**Liveness (`/healthz`)** answers “is this process running?” Orchestrators restart the container if this fails.

**Readiness (`/readyz`)** answers “can this instance take traffic?” Dependencies must ping. A dead realtime **subscription** is retried inside query-api and does **not** fail REST readiness.

PowerShell:

```powershell
curl.exe -sS http://127.0.0.1:8080/healthz
curl.exe -sS http://127.0.0.1:8080/readyz
curl.exe -sS http://127.0.0.1:8082/healthz
curl.exe -sS http://127.0.0.1:8082/readyz
```

Docker:

```powershell
docker compose -f infrastructure/docker-compose.yml ps
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml ps
```

Look for health `healthy` on services that define `healthcheck`. `depends_on` waiting for health is **not** a substitute for `/readyz` from the host.

---

# 20. Opening the dashboard

**URL (local):** http://127.0.0.1:3000/login

Anonymous visitors are redirected to `/login` by Next.js middleware (`apps/dashboard/src/middleware.ts`). Logged-in users hitting `/login` or `/signup` are sent to `/`.

### Login

1. Open `/login`.
2. Enter email and password.
3. Dashboard `POST /api/auth/login` forwards to query-api `POST /api/v1/auth/login`.
4. On success, Next.js sets cookie `pulselog_token` (httpOnly, SameSite=Lax, path `/`, maxAge 24h). The JSON response to the browser does **not** include the JWT.
5. You land on Overview (`/` or the `next` query param).

### Signup

1. Open `/signup`.
2. Organization name, email, password (**minimum 10 characters**).
3. `POST /api/auth/register` → `POST /api/v1/auth/register`.
4. Creates user, organization (role **owner**), and project **`default`**.
5. If `AUTH_SIGNUPS=false`, query-api returns **403** `signups are disabled`. Production Compose and Kubernetes default this to `false`. Local `.env.example` uses `true`.

### Seed demo user

After a **development** stack with signups enabled:

```powershell
.\scripts\seed-dashboard.ps1
```

This script registers or logs in `dashboard.demo@example.com` / `dashboard-demo-pass`, creates payment/auth/inventory services and keys, and sends events containing `ord-phase5`. Those credentials exist only because the seed script creates them; they are not a backdoor.

### Logout

Sidebar **Logout** calls `POST /api/auth/logout`, which forwards to query-api (JWT denylist) and clears the cookie.

> **NOTE:** If `COOKIE_SECURE=true` (or `NODE_ENV=production` without overriding) on **plain HTTP**, the browser will not store the cookie and login appears to fail. Local HTTP needs `COOKIE_SECURE=false`.

Screenshots are described in [docs/screenshots/README.md](screenshots/README.md) (`overview.png`, `logs.png`, `api-keys.png`) but are not required binaries in the repo.

---

# 21. Dashboard overview

Route `/`. Requires a selected project.

| UI | Meaning |
| --- | --- |
| **Total logs** | Count of events in the selected time window (`overview.total`). |
| **Errors** | `error + fatal` counts. Hint shows `error_rate` as a percentage of traffic. |
| **Warnings** | `warn` count. |
| **Active services** | Distinct services that emitted logs in the window. |
| **Logs over time** | Recharts volume chart. Built from four filtered `/stats/timeseries` calls (ERROR, WARN, INFO, DEBUG) plus an unfiltered series. Missing buckets are not invented. |
| **Error trend** | Error series from the same merged points. |
| **Service statistics** | Table from `/stats/services`, default sort `error_count`. |
| **Frequent / recent errors** | `/stats/errors` — groups by **exact** `message` text. |
| **Project selector** | Sidebar `<select>`; changing it reloads all Overview queries for that `project_id`. |
| **LIVE** | See [§24](#24-realtime-live-mode). Overview increments total/error/warn immediately, then debounces a REST refresh (~1.5s) for charts, services, and frequent errors. |
| **Time range** | 1h (1m buckets), 6h (5m), 24h (15m), 7d (1h). Default stats window if unset on API is last 24h. |
| **Refresh / poll** | Manual refresh; optional 10s / 30s / 60s polling. |

Empty charts mean no events in the window for that project (or ingest/processor not running).

---

# 22. Log explorer

Route `/logs`.

### Viewing logs

The table lists events **newest first** (Query API default). Columns include timestamp, level, service, and message. Click a row to open the detail drawer.

### Fields you will see

| Field | Meaning |
| --- | --- |
| Timestamp | Event time (`timestamp`) |
| Level | DEBUG, INFO, WARN, ERROR, FATAL |
| Service | Emitting service name (from the API key) |
| Message | Log body (substring search target) |
| Metadata | Optional map; full JSON in the drawer |
| Event ID | UUID assigned at ingest |

`ingested_at` is processor write time; shown in the drawer.

### Filters

| Filter | Query param | Behavior |
| --- | --- | --- |
| Time | `start` / `end` from the Refresh bar range | RFC3339 window |
| Service | `service` | Exact service name; dropdown from registered services |
| Level | `level` | DEBUG…FATAL |
| Search | `q` | Case-insensitive **substring** in `message` (max 256 chars on API). Debounced ~350ms in the UI. **Not** ranked full-text. |
| Event ID | `event_id` | Exact UUID |

Filters are combinable. REST filters run on the Query API. LIVE rows are filtered in the browser with the **same rules** (`matchesLiveFilter`).

### Cursor pagination / Load more

The first page is 50 rows (`page_size` default 50, max 200). If `has_more` is true, **Load more** sends the opaque `cursor` from `next_cursor`.

The cursor is base64url `RFC3339Nano|uuid` of the last row’s `(timestamp, event_id)`. This is **keyset** pagination: the database seeks the next key instead of `OFFSET 10000`. Keep the same filters when passing `cursor`. Do not construct cursors by hand.

Live-visible rows are capped at **200** in the explorer. REST “Load more” appends historical rows with `event_id` de-duplication.

---

# 23. Log details

Selecting a log opens a right-hand drawer (`LogDetail`). Escape or Close dismisses it.

| Field | Description |
| --- | --- |
| Event ID | UUID; Copy button |
| Timestamp | Event time |
| Ingested | Processor write time (`ingested_at`) when present |
| Level | Badge |
| Service | Monospace name |
| Project ID | When present on the event |
| Host | Optional |
| Trace ID | Optional (`trace_id` / ingest `traceId`) |
| Message | Full body |
| Metadata | Pretty-printed JSON, or “No structured metadata” |

---

# 24. Realtime LIVE mode

**LIVE** means the dashboard holds an authenticated WebSocket to query-api and renders `log.created` frames for the current project.

### Connection

1. User enables LIVE.
2. Browser `POST /api/auth/stream-ticket` (cookie) → query-api `POST /api/v1/stream/ticket`.
3. Ticket stored in Redis 45 seconds, one-time redeem (`GETDEL`).
4. Browser opens `QUERY_WS_PUBLIC_URL` (from `GET /api/runtime`) with `?ticket=...&project_id=...`.
5. Server redeems ticket, checks JWT denylist, verifies `HasProject`, then upgrades.
6. First frame: `{ "v": 1, "type": "hello", "project_id": "..." }`.
7. Later frames: `type: "log.created"` with `data` payload.

The long-lived JWT is **never** placed on the WebSocket URL.

### Status labels

| Status | Meaning |
| --- | --- |
| **Live** | Socket `onopen`; frames can flow |
| **Reconnecting** | Closed or ticket failed; retry with backoff **1s → 2s → … → 15s** (`nextReconnectDelay`) |
| **Disconnected** | LIVE off, or no project/session |

There is no tight reconnect loop.

### Pause / Resume / N new logs

**Pause live stream** keeps the socket, counts incoming events, and does not render them. The badge shows `{pending} new logs`. **Resume** flushes the buffer; explorer merge de-duplicates by `event_id`.

### Filters during realtime

Explorer applies service, level, `q`, and `event_id` to live events in the browser. Overview still bumps counters then refreshes REST for charts.

### Duplicate suppression

Merge uses `event_id` so a REST refresh plus a live frame of the same event does not double-render.

### Best-effort

Realtime is published **after** ClickHouse insert. Redis or a full hub buffer (256) can drop frames (`pulselog_ws_messages_dropped_total`). **ClickHouse/REST remains the historical source of truth.** Refresh and polling still work if the socket is down.

> **NOTE:** The Settings page still contains leftover copy saying live streaming is not enabled. LIVE is implemented (Phase 6). Use the LIVE toggle, not that Settings sentence.

---

# 25. Projects

A **project** is a tenant bucket for logs and API keys. Hierarchy:

```
Organization → Project → Service → Logs (ClickHouse project_id)
```

- Isolation is enforced by Query API (`project_id IN` the principal’s list) and by ingest keys bound to one project.
- Switching projects in the sidebar reloads Overview/Logs/Services for that id.
- **Owners** can create projects on `/projects` (`POST /api/v1/orgs/{id}/projects`). Admins, members, and viewers cannot.
- Logs from another project are not returned even if you guess `event_id` or pass a foreign `project_id` (forbidden / filtered).
- Pre-auth ClickHouse rows with zero UUID `project_id` are invisible to tenant queries.

There is no per-project role; org role applies to all projects in the org.

---

# 26. Services

A **service** is a named emitter inside a project, for example `payment-service`, `auth-service`, `inventory-service`.

- Registered in PostgreSQL (`UNIQUE (project_id, name)`).
- Each API key is bound to **one** service name. The ingest handler overwrites `service` from the key; a different client `service` is **403**.
- Services page lists the catalog and ClickHouse activity (`total`, `error_count`, `warn_count`, `error_rate`). Sort: `error_count` (default), `total`, `error_rate`.
- Owner and admin can create services. Member/viewer can view stats if they can query.

Create a service **before** creating a key for it.

---

# 27. API keys

**Owner** and **admin** only (`canManageKeys`).

1. Open **API Keys** (`/api-keys`).
2. Enter a name and choose a service.
3. Create key.
4. **Copy the raw token immediately.**
5. Store it in your app’s secret store / env (never git).
6. Send `X-API-Key` on ingest.
7. **Revoke** when compromised or unused. Ingest with that secret fails immediately (**401**).

Listed keys show `prefix`, name, service, created time, optional last used, revoked time. PostgreSQL stores **SHA-256(raw)** only.

> **WARNING: THE RAW API KEY IS DISPLAYED ONLY ONCE.**  
> PulseLog cannot show it again because the server never stores the plaintext. If you lose it, revoke and create a new key.

Format: `pl_live_` + 64 hexadecimal characters (32 random bytes).

---

# 28. Roles and permissions

Roles are defined **only** in `internal/auth/rbac.go`. There is no “Editor” role.

| Permission | Owner | Admin | Member | Viewer |
| --- | --- | --- | --- | --- |
| `org.manage` | Yes | | | |
| `members.manage` | Yes | | | |
| `projects.manage` | Yes | | | |
| `services.manage` | Yes | Yes | | |
| `apikeys.manage` | Yes | Yes | | |
| `logs.read` | Yes | Yes | Yes | Yes |

**Query API mapping**

| Action | Permission |
| --- | --- |
| List orgs / projects / services / logs / stats | `logs.read` |
| Create project | `projects.manage` (owner) |
| Add/update/remove members | `members.manage` (owner) |
| Create service | `services.manage` |
| Create/list/revoke API keys | `apikeys.manage` |

Member management endpoints exist on Query API only. The dashboard has **no invite UI**.

---

# 29. Ingestion API

| | |
| --- | --- |
| Method | `POST` |
| Paths | `/v1/logs` (single), `/v1/logs/batch` (array), `/ingest` (alias of single) |
| Host (local) | `http://127.0.0.1:8080` |
| Header | `X-API-Key: pl_live_...` (or `Authorization: Bearer pl_live_...`) |
| Content-Type | `application/json` |

**Single success (`202`):**

```json
{ "accepted": 1, "event_ids": ["11111111-1111-4111-8111-111111111111"] }
```

**Batch:** `{ "events": [ ... ] }` max 500. Response `accepted` + `event_ids`.

**Errors (typical):** `400` validation/invalid_json; `401` missing/invalid/revoked key; `403` service/project mismatch; `413` body too large; `429` rate limited; `503` kafka_unavailable / not_ready.

---

# 30. Send your first log

Replace `YOUR_API_KEY` with a key you created. Never commit real keys.

### PowerShell

```powershell
$body = @{
  service   = "payment-service"
  level     = "ERROR"
  message   = "Payment authorization failed"
  timestamp = "2026-08-29T09:00:00.000Z"
  host      = "ip-10-0-1-23"
  traceId   = "abc123"
  metadata  = @{ requestId = "req-1"; userId = "u-42" }
} | ConvertTo-Json -Compress -Depth 5

Invoke-RestMethod -Method POST -Uri "http://127.0.0.1:8080/v1/logs" `
  -Headers @{ "X-API-Key" = "YOUR_API_KEY" } `
  -ContentType "application/json" `
  -Body $body
```

### curl (Windows)

```powershell
curl.exe -sS -X POST http://127.0.0.1:8080/v1/logs `
  -H "Content-Type: application/json" `
  -H "X-API-Key: YOUR_API_KEY" `
  --data-binary "@event.json"
```

`event.json` should contain the JSON object in [§31](#31-log-schema).

If the dashboard does not show the event within a second or two, check processor health and ClickHouse; HTTP 202 only means Kafka accepted the produce.

---

# 31. Log schema

Canonical type: `internal/models/event.go`. JSON names as ingested.

| Field | Type | Required? | Description | Example |
| --- | --- | --- | --- | --- |
| `service` | string | Yes* | 1–128 chars `[A-Za-z0-9][A-Za-z0-9._-]*`. Overwritten from API key. Client value must match key or be omitted. | `payment-service` |
| `level` | string | Yes | Uppercased. `DEBUG` `INFO` `WARN` `ERROR` `FATAL` | `ERROR` |
| `message` | string | Yes | Trimmed; max 8192 chars | `Payment authorization failed` |
| `timestamp` | RFC3339 | No | Defaults to ingest time UTC | `2026-08-29T09:00:00.000Z` |
| `host` | string | No | Max 253 chars | `ip-10-0-1-23` |
| `traceId` | string | No | Max 128 chars; also read from metadata `traceId` / `trace_id` | `abc123` |
| `metadata` | object of strings | No | Max 32 keys; key 1–64 chars; value max 1024 | `{ "requestId": "req-1" }` |
| `event_id` | UUID string | No | Generated if missing; invalid UUID rejected | `11111111-1111-4111-8111-111111111111` |
| `project_id` | UUID string | No | Must match key if sent; otherwise stamped from key | *(from key)* |

\*Required by validation after bind; the key supplies `service` if you omit it, but sending a **wrong** service is forbidden.

---

# 32. Log levels

| Level | Typical use |
| --- | --- |
| `DEBUG` | Verbose diagnostics |
| `INFO` | Normal operations (“Payment captured”) |
| `WARN` | Recoverable problems (“retry scheduled”) |
| `ERROR` | Failed operation (“authorization failed”) |
| `FATAL` | Process-level failure; counted with errors in Overview |

Unknown levels fail validation. Overview “Errors” = ERROR + FATAL.

---

# 33. Integrating an application

There is **no official SDK** in this repository.

```
Your application
    → HTTP POST /v1/logs
    → PulseLog ingestion-api
```

Workflow:

1. Create a PulseLog project and service.
2. Create an API key for that service; store the secret in the app environment.
3. On interesting events, POST JSON. Use timeouts and retry on 503 (Kafka) / 429 (back off).
4. Do not treat 202 as ClickHouse persistence.
5. Optionally include `host`, `traceId`, and `metadata` for incident work.

Example (conceptual Node fetch):

```javascript
await fetch("http://127.0.0.1:8080/v1/logs", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "X-API-Key": process.env.PULSELOG_API_KEY,
  },
  body: JSON.stringify({
    service: "payment-service",
    level: "ERROR",
    message: "Payment authorization failed",
    metadata: { requestId: "req-1" },
  }),
});
```

Use your real ingest base URL in production (`https://ingest.pulselog.example.com`).

---

# 34. API overview

Base URLs: ingest `http://127.0.0.1:8080`, query `http://127.0.0.1:8082`. Dashboard browsers should use `/api/auth/*` and `/api/backend/*` on :3000, not raw query-api from JS (except WebSocket).

### Authentication (query-api)

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| POST | `/api/v1/auth/register` | None (if signups on) | Create user, org, default project |
| POST | `/api/v1/auth/login` | None (rate limited) | Issue JWT |
| POST | `/api/v1/auth/logout` | Bearer JWT | Denylist `jti` |
| GET | `/api/v1/auth/me` | Bearer JWT | Current principal |

### Organizations / members / projects / services / keys

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/api/v1/orgs` | JWT | List orgs for user |
| GET | `/api/v1/orgs/{orgID}/projects` | JWT + `logs.read` | List projects |
| POST | `/api/v1/orgs/{orgID}/projects` | JWT + `projects.manage` | Create project |
| GET | `/api/v1/orgs/{orgID}/members` | JWT + `logs.read` | List members |
| POST | `/api/v1/orgs/{orgID}/members` | JWT + `members.manage` | Add member |
| PATCH | `/api/v1/orgs/{orgID}/members/{userID}` | JWT + `members.manage` | Update role |
| DELETE | `/api/v1/orgs/{orgID}/members/{userID}` | JWT + `members.manage` | Remove member |
| GET | `/api/v1/projects/{projectID}/services` | JWT + `logs.read` | List services |
| POST | `/api/v1/projects/{projectID}/services` | JWT + `services.manage` | Create service |
| GET | `/api/v1/projects/{projectID}/api-keys` | JWT + `apikeys.manage` | List keys |
| POST | `/api/v1/projects/{projectID}/api-keys` | JWT + `apikeys.manage` | Create key (raw once) |
| DELETE | `/api/v1/api-keys/{keyID}` | JWT + `apikeys.manage` | Revoke key |

### Logs and statistics

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/api/v1/logs` | JWT | Search / paginate |
| GET | `/api/v1/logs/{eventID}` | JWT | Single event |
| GET | `/api/v1/stats/overview` | JWT | Totals and error rate |
| GET | `/api/v1/stats/timeseries` | JWT | Counts over time |
| GET | `/api/v1/stats/services` | JWT | Per-service stats |
| GET | `/api/v1/stats/errors` | JWT | Frequent ERROR messages |

List filters: `service`, `level`, `start`, `end`, `q`, `event_id`, `project_id`, `cursor`, `page_size`, `order` (`oldest` or default newest). Timeseries `interval`: `1m` `5m` `15m` `1h` `1d`.

### Realtime

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| POST | `/api/v1/stream/ticket` | JWT | One-time WS ticket |
| GET | `/api/v1/stream` | ticket query or Bearer | WebSocket upgrade |

### Ingest

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| POST | `/v1/logs` | API key | Single event |
| POST | `/v1/logs/batch` | API key | Batch |
| POST | `/ingest` | API key | Alias of single |

### Health and metrics

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/healthz` | None | Liveness (each Go service) |
| GET | `/readyz` | None | Readiness |
| GET | `/metrics` | None (keep private) | Prometheus |

### Dashboard BFF (Next.js :3000)

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/auth/login` | Set cookie |
| POST | `/api/auth/register` | Set cookie |
| POST | `/api/auth/logout` | Clear cookie + denylist |
| GET | `/api/auth/session` | Session from cookie |
| POST | `/api/auth/stream-ticket` | Proxy ticket |
| * | `/api/backend/{...}` | Proxy to `/api/v1/{...}` |
| GET | `/api/runtime` | `{ wsUrl }` |

Do not invent other endpoints; the tables above match `Handler()` registrations.

---

# 35. Authentication flow

### Humans (JWT)

1. Register or login with email/password.
2. Query-api verifies argon2id hash, issues HS256 JWT (`sub` = user id, `email`, `jti`, expiry `JWT_TTL`).
3. Dashboard stores JWT only in httpOnly cookie `pulselog_token`.
4. Subsequent BFF calls send `Authorization: Bearer`.
5. Logout: Redis `jwt:deny:{jti}` until token expiry; cookie cleared.
6. Replicas share the denylist via Redis; no sticky sessions required.

Login/register are rate limited (10/min/IP). Duplicate email on register → **409**.

### Applications (API keys)

Independent of JWT. Hash lookup in PostgreSQL. Revoked keys fail. Key stamps `project_id` and `service`. Used only on ingestion-api.

---

# 36. WebSocket protocol

| Item | Value |
| --- | --- |
| Ticket TTL | 45 seconds |
| Redeem | Redis `GETDEL` (one-time) |
| Origin | Must be in `CORS_ORIGINS` (non-prod also allows `http://127.0.0.1:3000` and `http://localhost:3000`) |
| Ping | Server ping every 20s; pong wait 60s |
| Payload cap | 16 KiB |
| Hub buffer | `WS_HUB_BUFFER` default 256; overflow drops |

**Hello**

```json
{ "v": 1, "type": "hello", "project_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" }
```

**log.created** (fields may omit empty optionals)

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

Unauthorized project → **403** before upgrade. Bad ticket → **401**. Rate limit → **429**. Too many connections → **503**.

Reconnect: dashboard fetches a **new** ticket each attempt (old ticket cannot be reused).

---

# 37. Prometheus

Optional Compose profile **`obs`**.

**Local development** (scrapes **host** Go processes via `host.docker.internal`):

```powershell
docker compose -f infrastructure/docker-compose.yml --profile obs up -d
# or: make up-obs
```

Prometheus: http://127.0.0.1:9090  
Config: `infrastructure/prometheus/prometheus.yml` (scrape interval 2s).

**Production-style Compose** (scrapes **container** names on the Docker network; ports bound to localhost):

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml --profile obs up -d
```

Config: `infrastructure/prometheus/prometheus.prod.yml` (15s). UI: http://127.0.0.1:9090

Do not publish `/metrics` on the public internet.

---

# 38. Grafana

Grafana is an **internal operations** UI. It is **not** the PulseLog product dashboard on :3000.

| Compose | URL | Auth |
| --- | --- | --- |
| Local `obs` | http://127.0.0.1:3001 | User `admin`; password is the local Compose placeholder `pulselog_dev_only`; anonymous Viewer **enabled** |
| Prod-like `obs` | http://127.0.0.1:3001 | Anonymous **off**; set `GRAFANA_ADMIN_PASSWORD` in `.env.prod` |

Provisioned dashboard title: **PulseLog operations** (`infrastructure/grafana/dashboards/pulselog-ops.json`): ingest rate/latency, processor throughput, Kafka lag, ClickHouse write latency, query latency, errors/rejects, WebSocket/realtime.

---

# 39. Metrics

All names are Prometheus metrics from `internal/metrics/metrics.go`. Labels are coarse (`kind`, `reason`, `scope`) — not user IDs or keys.

### HTTP

- `pulselog_http_requests_total` — count by method/path/status
- `pulselog_http_request_duration_seconds` — latency histogram
- `pulselog_http_requests_in_flight` — gauge

### Ingest / Kafka produce

- `pulselog_ingest_events_accepted_total`
- `pulselog_ingest_events_rejected_total{reason}`
- `pulselog_kafka_produce_duration_seconds`
- `pulselog_kafka_produce_errors_total`

### Processor

- `pulselog_processor_events_consumed_total`
- `pulselog_processor_events_written_total`
- `pulselog_processor_events_failed_total{reason}`
- `pulselog_processor_events_retried_total`
- `pulselog_processor_events_dlq_total{reason}`
- `pulselog_processor_clickhouse_write_duration_seconds`
- `pulselog_processor_batch_size`
- `pulselog_kafka_consumer_lag`

### Query / ClickHouse reads

- `pulselog_clickhouse_query_duration_seconds{op}`
- `pulselog_clickhouse_query_errors_total{op}`

### Auth / abuse

- `pulselog_auth_success_total{kind}`
- `pulselog_auth_failures_total{kind,reason}`
- `pulselog_api_key_rejected_total{reason}`
- `pulselog_rate_limited_total{scope}`
- `pulselog_authz_denied_total{reason}`

### Realtime

- `pulselog_realtime_published_total`
- `pulselog_realtime_publish_errors_total`
- `pulselog_realtime_subscribe_errors_total`
- `pulselog_ws_connections`
- `pulselog_ws_connects_total`
- `pulselog_ws_auth_failures_total`
- `pulselog_ws_messages_delivered_total`
- `pulselog_ws_disconnects_total`
- `pulselog_ws_messages_dropped_total`

---

# 40. Backend tests

From repo root:

```powershell
go test ./internal/... ./services/...
```

Makefile: `make test` (same packages). CI also runs `go test ./cmd/...`, `gofmt`, and `go vet`.

Processor tests use in-memory ClickHouse/DLQ stubs. Query API tests mock the store. **Neither requires Kafka or ClickHouse.**

`make fmt` / `make vet` for local hygiene.

---

# 41. Dashboard tests

```powershell
cd apps/dashboard
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```

| Script | Tool |
| --- | --- |
| `npm run lint` | ESLint |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Vitest (`vitest run`) |
| `npm run build` | `next build --turbopack` |
| `npm run dev` | `next dev --turbopack` |

CI uses `npm ci` then those four quality steps.

---

# 42. Full regression testing

Recommended before a commit or release:

1. `gofmt` / `go vet` / `go test ./internal/... ./services/... ./cmd/...`
2. Dashboard `lint`, `typecheck`, `test`, `build`
3. `docker compose -f infrastructure/docker-compose.yml config`
4. `docker compose --env-file .env.prod.example -f infrastructure/docker-compose.prod.yml config`
5. Optional: kustomize build / kubeconform (CI images)
6. Optional smoke: Compose up, `/readyz`, seed or one ingest, dashboard login
7. Optional k6 if you changed ingest/query/live paths

GitHub Actions runs the enforced subset on every PR and `main` ([§55](#55-github-actions)).

---

# 43. Performance testing

[k6](https://k6.io) drives HTTP and WebSocket load. JS scenarios live in `tests/load/`. PowerShell wrappers live in `scripts/load/`.

| Wrapper | k6 script | What it does |
| --- | --- | --- |
| `scripts/load/setup.ps1` | — | Creates bench user/keys; writes `tests/load/.credentials.json` (gitignored) |
| `scripts/load/ingest.ps1` | `ingest.js` | Constant-arrival ingest |
| `scripts/load/query.ps1` | `query.js` | Query/stats mix |
| `scripts/load/mixed.ps1` | `mixed.js` | Ingest + query VUs + WS clients |
| `scripts/load/ws.ps1` | `ws.js` | Ticket auth WebSocket hold |
| `scripts/load/rate-limit.ps1` | `rate-limit.js` | Expect 429s at default 120/min |
| `scripts/load/ws-slow.ps1` | — | Slow-subscriber buffer experiment |
| `scripts/load/failures.ps1` | — | Dependency stop/start probes |
| `scripts/load/env-info.ps1` | — | Machine snapshot JSON |
| `scripts/load/dataset.ps1` | — | Extra seed helper |

Credentials are never hardcoded in JS. `make load-setup` / `make load-test` (100/s, 20s ingest) call the PowerShell files.

Events include `metadata.run_id` and message prefix `bench:{RUN_ID}`. API keys are **per service**.

---

# 44. Running a benchmark

1. Start local infra: `docker compose -f infrastructure/docker-compose.yml up -d`
2. Start host ingest, processor, query-api.
3. For **throughput** tests, raise limits **only in those process terminals**:

```powershell
$env:RATE_LIMIT_INGEST="1000000"
$env:RATE_LIMIT_QUERY="1000000"
$env:CORS_ORIGINS="http://127.0.0.1:3000,http://localhost:3000"
```

4. Keep `.env.example` at 120/min.

> **WARNING:** Benchmark rate limits must **NOT** become production defaults.

5. Run:

```powershell
.\scripts\load\setup.ps1
.\scripts\load\env-info.ps1
.\scripts\load\ingest.ps1 -Rate 100 -Duration 20s
.\scripts\load\query.ps1
.\scripts\load\mixed.ps1
.\scripts\load\ws.ps1 -Clients 1
```

6. Run `.\scripts\load\rate-limit.ps1` **only** while ingest still uses 120/min (not the 1_000_000 override).

k6 is a native `k6` binary or `docker run grafana/k6`. Summaries go under `tests/load/results/` (gitignored).

---

# 45. Existing performance results

These results were measured on a local development machine and should not be interpreted as production capacity.

Full tables, failure tests, and reproduce commands: [docs/PERFORMANCE.md](PERFORMANCE.md).

Collected 2026-08-30 with host Go services plus Compose Kafka, ClickHouse, PostgreSQL, Redis.

| Measurement | Result on that machine |
| --- | --- |
| Ingest 500 events/s | 9990/9990 HTTP 202; p50 12.5 ms; p95 15.6 ms; p99 21.8 ms |
| Ingest ~1000 events/s | 19714/19714 HTTP 202; ~982/s; p50 14.1 ms; p95 116 ms |
| Local saturation region | ~1000–1200 ingest/s (latency / k6 VUs), not a cluster rating |
| Query API ~105k rows, 8 VUs | 0% errors; HTTP p50 44 ms; p95 75 ms; p99 93 ms |
| Processor vs Kafka | consumed = written in the baseline window; lag 0 after runs |
| Mixed 200 ingest/s + queries + 3 WS | 100% checks; HTTP p95 ~56 ms; lag 0 after |
| ClickHouse stopped | Ingest still 202; processor/query not ready; exhausted batch → DLQ |
| Processor stopped then restarted | Lag drained in ~1.6 s; rows appeared in ClickHouse |

p50 / p95 / p99 are latency percentiles (median, 95th, 99th). See [§76](#76-glossary).

---

# 46. Understanding Docker in PulseLog

| Concept | Plain meaning | In PulseLog |
| --- | --- | --- |
| **Image** | Immutable filesystem + default command | `pulselog/ingestion-api:local`, GHCR tags, `apache/kafka:3.9.0`, … |
| **Container** | Running instance of an image | `pulselog-kafka-1`, `pulselog-query-api-1`, … |
| **Volume** | Disk that survives `compose down` | `kafka_data`, `clickhouse_data`, `postgres_data`, `redis_data` |
| **Network** | Private DNS between containers | Local default network; prod `pulselog_edge` + `pulselog_data` |
| **Compose** | YAML describing many containers | `infrastructure/docker-compose.yml` and `.prod.yml` |

---

# 47. PulseLog containers

### Local `infrastructure/docker-compose.yml`

| Service | Purpose | Host port | Public? | Persistent data? |
| --- | --- | --- | --- | --- |
| `kafka` | KRaft broker | 9092 | Dev host only | `kafka_data` |
| `kafka-data-init` | chown volume uid 1000 | — | No | — |
| `kafka-init` | Create topics | — | No | — |
| `clickhouse` | Log store | 8123, 9000 | Dev host | `clickhouse_data` |
| `postgres` | Identity | 5432 | Dev host | `postgres_data` |
| `redis` | Limits / live | 6379 | Dev host | `redis_data` (AOF) |
| `migrate` | Schema job (`app` profile) | — | No | — |
| `ingestion-api` | Ingest (`app`) | 8080 | Dev | No |
| `log-processor` | Worker (`app`) | 8081 | Dev | No |
| `query-api` | REST+WS (`app`) | 8082 | Dev | No |
| `dashboard` | UI (`app`) | 3000 | Dev | No |
| `prometheus` | Scrape (`obs`) | 9090 | Dev | No (24h TSDB in container) |
| `grafana` | Ops UI (`obs`) | 3001 | Dev | Provisioned files |

### Production-style `docker-compose.prod.yml`

Same data services **without** published 5432/6379/9092/8123/9000/8081. Published: **8080**, **8082**, **3000**. `proxy` profile: **80/443**. `obs`: **127.0.0.1:9090** and **127.0.0.1:3001**.

---

# 48. Docker volumes

Named volumes: `kafka_data`, `clickhouse_data`, `postgres_data`, `redis_data`, and prod `caddy_data`.

| Volume | If deleted |
| --- | --- |
| `clickhouse_data` | Historical logs gone |
| `postgres_data` | Users, orgs, keys gone |
| `kafka_data` | In-flight messages and offsets gone |
| `redis_data` | Rate windows, denylist, tickets gone (logs remain in CH) |

`compose down` **without** `-v` keeps volumes. See the [§17](#17-stopping-pulselog) warning.

---

# 49. Useful Docker commands

Assume repo root. Substitute the prod `-f` / `--env-file` pair when using production-style Compose.

```powershell
docker ps
docker compose -f infrastructure/docker-compose.yml ps
docker compose -f infrastructure/docker-compose.yml logs
docker compose -f infrastructure/docker-compose.yml logs -f
docker compose -f infrastructure/docker-compose.yml logs -f --tail=100
docker compose -f infrastructure/docker-compose.yml restart
docker compose -f infrastructure/docker-compose.yml restart clickhouse
docker compose -f infrastructure/docker-compose.yml stop
docker compose -f infrastructure/docker-compose.yml down
docker images
docker volume ls
```

Prod:

```powershell
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml ps
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml logs -f query-api
```

Makefile `make logs` follows local Compose logs.

---

# 50. Troubleshooting guide

### Docker says virtualization not detected

**Cause:** BIOS virtualization off or Windows feature missing.  
**Check:** Task Manager → Performance → CPU “Virtualization”; `wsl --status`.  
**Fix:** Enable VT-x/AMD-V; enable Virtual Machine Platform; reboot. See [§51](#51-wsl--docker-recovery).

### WSL2 cannot start

**Cause:** Distro not installed, WSL1, or Windows update pending.  
**Check:** `wsl --list --verbose`.  
**Fix:** `wsl --install` / `wsl --set-default-version 2`. Avoid deleting distros as a first step.

### Docker Engine stopped

**Cause:** Docker Desktop not running after reboot or crash.  
**Check:** `docker info`.  
**Fix:** Start Docker Desktop; wait until `docker info` shows a Server.

### Port 8082 connection refused

**Cause:** query-api not running, or prod Compose not up.  
**Check:** `curl.exe http://127.0.0.1:8082/healthz`; `docker compose ... ps`.  
**Fix:** Start query-api (`go run` or container). Confirm nothing else bound 8082 (`netstat -ano | findstr 8082`).

### Dashboard registration returns 500 / 403

**Cause:** query-api down; `AUTH_SIGNUPS=false`; Postgres down; validation error.  
**Check:** query-api logs; `/readyz`; signup response body.  
**Fix:** Enable signups only if you intend open registration; otherwise create users via API with signups on a private network.

### Kafka unavailable (ingest 503 `kafka_unavailable`)

**Cause:** Broker down or wrong `KAFKA_BROKERS` (host `localhost:9092` vs Docker `kafka:19092`).  
**Check:** ingest `/readyz`; Kafka healthcheck; `docker compose logs kafka`.  
**Fix:** Start Kafka; use the address that matches **where the process runs**.

### ClickHouse unavailable

**Cause:** Container unhealthy; wrong `CLICKHOUSE_ADDR`/`PASSWORD`.  
**Check:** processor/query `/readyz`; `docker compose logs clickhouse`.  
**Fix:** Wait for health; align env with Compose user/password.

### Redis unavailable

**Cause:** Redis down.  
**Check:** ingest/query `/readyz` 503; processor still ready.  
**Fix:** Start Redis. Ingest HTTP may still 202 (rate limit fail-open) while `/readyz` fails.

### Dashboard cannot connect to Query API

**Cause:** Wrong `QUERY_API_URL`; query-api not ready; cookie/CORS.  
**Check:** From dashboard container, URL is `http://query-api:8082`. Host `npm run dev` uses `http://127.0.0.1:8082`.  
**Fix:** Match env to topology.

### WebSocket disconnected

**Cause:** Bad `QUERY_WS_PUBLIC_URL`, CORS origin, Redis tickets, cookie, or query-api restart.  
**Check:** Browser network WS; query-api logs; Redis ping.  
**Fix:** Set public `ws://` or `wss://` URL as the **browser** sees it; include dashboard origin in `CORS_ORIGINS`.

### 403 API key

**Cause:** `service` or `project_id` does not match the key.  
**Check:** Key’s service name vs JSON `service`.  
**Fix:** Omit `service` or send the exact registered name.

### 401 authentication

**Cause:** Missing/invalid/revoked API key; missing/expired/denylisted JWT.  
**Fix:** New key; login again; do not put JWT on the WS URL.

### 429 rate limit

**Cause:** Default 120 ingest/min/key or 10 login/min/IP.  
**Fix:** Slow down; do not raise production limits to bench values.

### CORS error

**Cause:** Origin not in `CORS_ORIGINS`; `*` is ignored.  
**Fix:** List exact origins (`http://127.0.0.1:3000,http://localhost:3000`).

### Secure cookie prevents login on local HTTP

**Cause:** `COOKIE_SECURE=true` or production `NODE_ENV` without override.  
**Fix:** `COOKIE_SECURE=false` on HTTP.

### Port already in use

**Cause:** Old `go run` or another stack.  
**Fix:** Stop the old process or change `HTTP_ADDR` / `QUERY_HTTP_ADDR`.

### Container unhealthy

**Cause:** `/readyz` failing inside the container.  
**Check:** `docker compose ps`; `docker compose logs <service>`.  
**Fix:** Fix the dependency named in the log (`kafka`, `clickhouse`, `dependency is unreachable`).

### Migration failure

**Cause:** SQL error or lock.  
**Check:** `docker compose logs migrate`; process exit on ingest/query start.  
**Fix:** Fix DSN/permissions; do not run conflicting schema changes by hand. Migrations are additive `IF NOT EXISTS`.

### Dashboard does not show a newly ingested log

**Cause:** 202 only queued Kafka; processor lag; wrong project; filters; TTL; zero `project_id`.  
**Check:** processor metrics consumed vs written; ClickHouse; project selector; time range.  
**Fix:** Ensure processor is running; wait for flush (≤500ms or 100 events); Refresh.

---

# 51. WSL / Docker recovery

Work **top-down**. Do not delete VMs or `wsl --unregister` as a first fix.

```powershell
wsl --status
wsl --list --verbose
docker info
docker version
```

1. Confirm **Virtual Machine Platform** is enabled.
2. Start Docker Desktop; wait until it reports running.
3. If `docker info` cannot talk to the engine, restart Docker Desktop from the tray.
4. If WSL shows version 1, `wsl --set-version <distro> 2`.
5. Only after that, consider Windows “Repair” Docker Desktop from Apps.

Avoid `docker system prune -a --volumes` unless you accept losing **all** local images and volumes.

---

# 52. Reading logs

Go services log structured JSON to stdout (`service`, `env`, `level`, `request_id`). They must not log raw API keys, passwords, JWTs, or cookies.

```powershell
# Local Compose
docker compose -f infrastructure/docker-compose.yml logs -f kafka
docker compose -f infrastructure/docker-compose.yml logs -f clickhouse

# App profile / prod Compose
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml logs -f ingestion-api
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml logs -f log-processor
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml logs -f query-api
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml logs -f dashboard
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml logs migrate
```

Host `go run` / `npm run dev`: read the terminal that started the process.

---

# 53. Security architecture

| Control | Implementation |
| --- | --- |
| Password hashing | argon2id PHC string; min 10 chars |
| JWT | HS256, `jti`, TTL, Redis denylist on logout |
| API keys | `pl_live_` + random; SHA-256 at rest; prefix for display |
| RBAC | Org roles in `internal/auth` |
| Project isolation | Key stamp + query `project_id` filter; WS `HasProject` before upgrade |
| Rate limiting | Redis fixed window; fail-open on Redis INCR error |
| WebSocket tickets | 45s, one-time, not the JWT |
| Origin validation | CORS allow-list; `*` dropped; WS `CheckOrigin` |
| Cookies | httpOnly, SameSite=Lax, Secure when HTTPS/`COOKIE_SECURE` |
| Security headers | nosniff, DENY frame, no-referrer, no-store |
| Non-root containers | Go uid 65532; dashboard 1001; no privileged Compose/K8s |
| Secrets | Env / Compose env-file / K8s Secret; not baked into images (`.dockerignore` excludes `.env*`) |
| Network isolation | Prod `pulselog_data` internal; only edge publishes 3000/8080/8082 |
| Signup | `AUTH_SIGNUPS=false` in prod templates |

---

# 54. Production security checklist

Aligned with [docs/DEPLOYMENT.md](DEPLOYMENT.md):

- [ ] Replace placeholder `JWT_SECRET`
- [ ] Replace PostgreSQL and ClickHouse passwords; keep DSN in sync
- [ ] Set explicit `CORS_ORIGINS` (HTTPS dashboard origin); never `*`
- [ ] Set `QUERY_WS_PUBLIC_URL` to `wss://...`
- [ ] `COOKIE_SECURE=true` behind HTTPS
- [ ] `AUTH_SIGNUPS=false` unless you have an invite process
- [ ] `ENV=production`
- [ ] Rate limits stay at 120/min ingest/query, 10/min login — not bench values
- [ ] TLS at Caddy / LB / Ingress
- [ ] Firewall: 80/443 (and SSH) only on a demo VM
- [ ] Do not publish PostgreSQL, Redis, Kafka, ClickHouse, :8081, `/metrics`, Prometheus, Grafana
- [ ] Protect Grafana (no anonymous, real admin password); bind to localhost or SSO
- [ ] `.env.prod` mode 600, not in git
- [ ] Volume or managed backups for Postgres + ClickHouse
- [ ] Images by digest/SHA, not `latest` alone

---

# 55. GitHub Actions

### CI — `.github/workflows/ci.yml`

**Triggers:** pull request; push to `main`; `workflow_call` from publish.

| Job | What |
| --- | --- |
| `go` | `gofmt` clean, `go vet`, `go test` for internal/services/cmd — **enforced**. Go **1.25.x** |
| `dashboard` | Node **20**, `npm ci`, lint, typecheck, test, build — **enforced** |
| `docker` | Build five images — **enforced** (needs go + dashboard) |
| `infra` | Compose config (local + prod example env), kustomize build, kubeconform — **enforced** |
| `security` | gitleaks **enforced**; govulncheck, npm audit, Trivy **advisory** (`continue-on-error`) |

### Publish — `.github/workflows/publish.yml`

**Triggers:** push to `main`; tags `v*.*.*`.

Runs CI via `workflow_call`, then builds/pushes each image to GHCR. Does not deploy to a cluster.

---

# 56. Container registry

Registry: **GitHub Container Registry** (`ghcr.io`). Image name:

`ghcr.io/<github.repository>/<service>`

Example shape used in Kubernetes YAML: `ghcr.io/pulselog/pulselog/ingestion-api:main` (placeholder org; your fork will differ).

**Tags** (`docker/metadata-action`):

- Git commit **SHA** (no `sha-` prefix in the workflow `type=sha,prefix=`)
- Branch name (`main`)
- Semver for tags: `{{version}}` and `{{major}}.{{minor}}`

**Why not `latest` only:** rolling `latest` makes rollbacks ambiguous. Deploy a SHA or version tag. See [docs/DEPLOYMENT.md](DEPLOYMENT.md).

---

# 57. Production-style deployment

Summary of [docs/DEPLOYMENT.md](DEPLOYMENT.md). Read that document for networking, TLS, backups, rollback, and cloud-neutral steps.

- Application services are **stateless**.
- Durable state: PostgreSQL (identity), ClickHouse (logs), Kafka (in-flight), Redis (ephemeral).
- Local dev: Compose infra + `go run` / `npm run dev`.
- Portfolio/demo: `docker-compose.prod.yml` on one VM.
- Serious HA: managed Kafka, ClickHouse, PostgreSQL, Redis, multiple app replicas, TLS at the edge.

This repository **does not provision paid cloud resources**.

---

# 58. Kubernetes

Directory `deploy/kubernetes/` (Kustomize). **Application workloads only.**

| File | Role |
| --- | --- |
| `namespace.yaml` | Namespace `pulselog` |
| `configmap.yaml` | Non-secret env (`ENV=production`, topics, TTL, `AUTH_SIGNUPS=false`, CORS placeholders, …) |
| `secret.example.yaml` | **Template only** — not in `kustomization.yaml` |
| `migrate-job.yaml` | Job `pulselog-migrate` |
| `ingestion-api.yaml` | Deployment replicas **2**, Service 8080, probes `/readyz` `/healthz`, resources, non-root |
| `log-processor.yaml` | Replicas **1**, port 8081, longer termination (25s) |
| `query-api.yaml` | Replicas **2**, port 8082 |
| `dashboard.yaml` | Replicas **2**, port 3000, probes `/login`, user 1001 |
| `ingress.yaml` | nginx class; hosts pulselog/api/ingest.example.com; WS timeouts 3600s; TLS secret `pulselog-tls` |
| `kustomization.yaml` | Lists resources; omits the example secret |

Images default to `ghcr.io/pulselog/pulselog/<svc>:main`. Replace for your registry.

```bash
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl -n pulselog apply -f deploy/kubernetes/secret.example.yaml   # replace values first
kubectl apply -k deploy/kubernetes
```

**Kafka, PostgreSQL, Redis, and ClickHouse are recommended as managed services** for production. The manifests do not self-host them.

---

# 59. Portfolio deployment

**Portfolio/demo deployment, not recommended HA production architecture.**

One VM + Docker Compose + optional Caddy + named volumes is enough to show the product. It is a single failure domain (one Kafka broker, RF=1, local disks).

High-availability production: multiple brokers (RF≥3), managed data stores, multiple replicas, TLS, backups, no public data ports.

See [docs/DEPLOYMENT.md](DEPLOYMENT.md) “Portfolio Deployment”.

---

# 60. Scaling

| Component | How it scales | Constraint |
| --- | --- | --- |
| ingestion-api | Horizontal replicas | Kafka produce + Postgres/Redis connections |
| log-processor | One useful member per `logs-ingest` partition | Local **6** partitions ⇒ max 6 useful replicas |
| query-api | Horizontal replicas | Redis pub/sub fans out live events; no sticky WS |
| dashboard | Stateless replicas | BFF to query-api; JWT cookies |

Processor replicas beyond partition count idle. ClickHouse/Postgres/Kafka capacity is the real data-plane limit.

---

# 61. Data storage

| Store | What |
| --- | --- |
| **PostgreSQL** | Users, password hashes, orgs, projects, services, memberships, API key hashes/prefixes, audit events |
| **ClickHouse** | Log events and `logs_per_minute` rollups |
| **Kafka** | Pipeline buffer + DLQ; not the archive |
| **Redis** | Rate-limit keys, JWT denylist, WS tickets, pub/sub (ephemeral) |

---

# 62. Retention

`CLICKHOUSE_TTL_DAYS` default **90** — raw logs expire.  
`CLICKHOUSE_MV_TTL_DAYS` default **180** — per-minute counts expire.

ClickHouse drops expired parts in the background. Increasing/decreasing env applies `MODIFY TTL` on migrate/processor/query startup.

Expired logs are **gone** from search. Kafka retention is a separate broker setting (replay window, not years of archive).

---

# 63. Backup

| Context | Expectation |
| --- | --- |
| Local development | Volumes are convenience; `down -v` wipes them. Snapshot only if you care. |
| Portfolio VM | Backup Postgres + ClickHouse volume directories on a schedule. |
| Production | Managed Postgres (PITR); ClickHouse snapshots; Kafka sized for replay only; Redis optional |

Recovery: restore PostgreSQL then ClickHouse; replay leftover Kafka only if you accept at-least-once duplicates.

---

# 64. Routine maintenance

- Hit `/healthz` and `/readyz` on published APIs.
- Watch disk for Docker volumes / ClickHouse.
- `docker compose ps` / `docker stats`.
- Inspect Kafka lag: `pulselog_kafka_consumer_lag` or `kafka-consumer-groups.sh --group log-processor --describe`.
- Watch processor DLQ counters and ClickHouse write errors.
- Rotate secrets and API keys.
- Run test suites before upgrades.
- Keep image tags pinned; rebuild on Go/Node base updates.
- Confirm backups actually restore on a copy.

---

# 65. Updating PulseLog

1. `git pull` (or checkout the release tag).
2. Run tests ([§42](#42-full-regression-testing)).
3. Rebuild images (`make images` or `compose up -d --build`).
4. Run **migrate** before or as Compose `depends_on` already does (`service_completed_successfully`). Kubernetes: apply the migrate Job before rolling Deployments.
5. Restart / roll app replicas.
6. Schema migrations are **additive** (`IF NOT EXISTS`). There is no automatic down-migration. Roll back by image digest if needed.

---

# 66. Rebuilding containers

Rebuild when Go/TS/Dockerfiles/dependencies change:

```powershell
make images
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml up -d --build
```

Individual (from [docs/DEPLOYMENT.md](DEPLOYMENT.md)):

```powershell
docker build -f services/ingestion-api/Dockerfile -t pulselog/ingestion-api:local .
docker build -f services/log-processor/Dockerfile -t pulselog/log-processor:local .
docker build -f services/query-api/Dockerfile -t pulselog/query-api:local .
docker build -f cmd/migrate/Dockerfile -t pulselog/migrate:local .
docker build -f apps/dashboard/Dockerfile -t pulselog/dashboard:local apps/dashboard
```

Infra-only image bumps (Kafka/CH) use `compose pull` then `up -d`.

---

# 67. Making code changes

Typical flow:

1. Branch from `main`.
2. Change code (see §68–70).
3. `go test` and/or dashboard `npm test`.
4. Optionally `docker build` the affected image.
5. Commit (when you intend to).
6. Open a PR; CI must be green.
7. Merge; publish workflow tags GHCR on `main` / version tags.

---

# 68. Adding a backend feature

| Kind of change | Typical place |
| --- | --- |
| Ingest validation / JSON | `internal/models`, `services/ingestion-api` |
| Kafka produce/consume/DLQ | `internal/kafka`, processor `consume.go` / `process.go` |
| ClickHouse schema / writes | `internal/clickhouse`, `infrastructure/clickhouse/init.sql` |
| Query SQL / pagination | `services/query-api/query.go`, `store.go` |
| Auth / RBAC / keys | `internal/auth`, `internal/identity`, `services/query-api/admin.go` |
| Live protocol | `internal/realtime`, `services/query-api/stream.go` |
| Config | `internal/config`, `.env.example`, `.env.prod.example` |
| Metrics | `internal/metrics` |
| HTTP middleware | `internal/httpx` |

Shared logic belongs in `internal/` so ingest, processor, and query stay consistent.

---

# 69. Adding dashboard features

| Area | Path |
| --- | --- |
| Routes | `apps/dashboard/src/app/(app)/` and `(auth)/` |
| BFF | `src/app/api/auth/*`, `api/backend/[...path]`, `api/runtime` |
| API client | `src/lib/api.ts` — do not scatter raw `fetch` to query-api |
| Types | `src/lib/types.ts` |
| Live helpers | `src/lib/live.ts`, `components/live-provider.tsx` |
| Charts / table | `components/charts.tsx`, `log-table.tsx`, `log-detail.tsx` |
| Auth cookie | `src/lib/cookies.ts`, `session.ts`, `middleware.ts` |

Follow existing pages: `useApp()` for project/role, `api.*` for data, tests beside pages (`*.test.tsx`).

---

# 70. Adding database changes

**PostgreSQL:** add `internal/postgres/migrations/002_....sql`. `Migrate` applies new files by version into `schema_migrations` under advisory lock. ingestion-api and query-api also migrate on start; Compose/K8s still run `cmd/migrate` first.

**ClickHouse:** add idempotent SQL to `EnsureSchema` **and** `infrastructure/clickhouse/init.sql` (`CREATE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `MODIFY TTL`). Fresh volumes use init SQL; existing volumes rely on `EnsureSchema`.

There are no down-migrations. Additive only.

---

# 71. Important engineering decisions

**Kafka instead of writing ClickHouse in the ingest handler.**  
Plain: the HTTP request should return quickly even if the database is slow.  
Technical: produce to `logs-ingest` with acks=1; processor batches inserts; ingest 503s only if Kafka itself is down.

**ClickHouse instead of PostgreSQL for logs.**  
Plain: counting millions of rows by minute and service is what column stores do.  
Technical: MergeTree, daily partitions, ORDER BY matching dashboard filters, token bloom on `message`, TTL, SummingMergeTree MV.

**PostgreSQL for identity.**  
Plain: users and keys are relational and low volume.  
Technical: unique emails, FK from keys to services/projects, hashed secrets.

**Redis for rate limits and fanout.**  
Plain: every replica must share “too many requests” and live events.  
Technical: fixed-window INCR+EXPIRE; pub/sub channels namespaced by project UUID; tickets GETDEL.

**WebSockets for realtime.**  
Plain: polling every second wastes work; a socket pushes new rows.  
Technical: ticket auth so JWT is not in query strings; hub buffer drops slow clients; REST fallback remains.

**Cursor / keyset pagination.**  
Plain: page 1000 of OFFSET is slow.  
Technical: seek on `(timestamp, event_id)`; opaque cursor.

**Bounded WebSocket buffers.**  
Plain: a slow tab must not pin memory forever.  
Technical: default 256; `pulselog_ws_messages_dropped_total`.

**Best-effort realtime.**  
Plain: missing a live frame is OK if history is in ClickHouse.  
Technical: Redis publish after INSERT; failure does not block Kafka commit.

**Docker.**  
Plain: Kafka/ClickHouse are painful to install natively on Windows.  
Technical: Compose for demo; same Dockerfiles for CI/GHCR/K8s.

---

# 72. Delivery guarantees

PulseLog does **not** claim exactly-once processing.

| Stage | Behavior |
| --- | --- |
| Kafka produce | `RequireOne` ack; client sees 202 after produce |
| Processor consume | At-least-once; commit after success path |
| ClickHouse insert | Batch insert; MergeTree has **no unique constraint** |
| Offset commit | After INSERT success **or** DLQ publish success |
| Duplicate window | INSERT succeeded, process dies before `CommitMessages` → same Kafka record consumed again → second row, same `event_id` |
| In-batch dedupe | Same `event_id` skipped only within one processor batch |
| DLQ | Poison JSON/validation; exhausted ClickHouse writes (`clickhouse_write_failed`). DLQ publish failure → no commit |
| Realtime | Best-effort after insert |

Prefer at-least-once over dropping a payment error. Query with `LIMIT 1 BY event_id` when uniqueness matters. ORDER BY includes `event_id` so a later `ReplacingMergeTree(ingested_at)` could collapse duplicates without a key redesign.

**Do not treat HTTP 202 alone as proof of ClickHouse persistence.**

---

# 73. Current limitations

From README, PERFORMANCE, DEPLOYMENT, and source (still true):

- Open registration when `AUTH_SIGNUPS=true` (local default). Prod Compose/K8s default `false`
- No email verification, MFA, or password-reset flow
- JWT HMAC secret is a local placeholder unless you set `JWT_SECRET`
- Single-broker Compose Kafka (RF=1) is not a production cluster
- Compose volumes are not a backup/HA story
- TLS terminates at proxy/LB/Ingress, not inside each Go service
- Kubernetes manifests do not self-host Kafka, ClickHouse, PostgreSQL, or Redis
- Org-level roles apply to every project (no per-project ACL)
- Pre-Phase-4 ClickHouse rows with zero `project_id` are invisible to tenant queries
- Live updates are best-effort; Redis/WS loss does not rewind REST
- Live explorer filters in the browser; it is not a second query engine
- Org member invite UI is not in the dashboard (Query API only)
- Message search is substring, not ranked full-text
- Common errors group by exact `message` text
- At-least-once delivery can duplicate `event_id` rows
- Rate limits are fixed-window, not token-bucket
- Slow WebSocket subscribers may drop messages (buffer 256)
- Local benchmark numbers are not production capacity
- No paid cloud is provisioned from this repo
- Settings page copy about live streaming is stale (LIVE exists)

---

# 74. Frequently asked questions

**Does Docker need to be running?**  
Yes for Kafka, ClickHouse, PostgreSQL, and Redis. Production-style Compose also runs the apps in Docker.

**Can PulseLog run without Docker?**  
Not as documented. You would have to install Kafka, ClickHouse, PostgreSQL, and Redis yourself and point env vars at them.

**Does Docker keep running after I close Cursor?**  
Yes, while Docker Desktop is running.

**Does PulseLog keep running after I close Cursor?**  
Containers yes. `go run` and `npm run dev` no.

**What happens after restarting Windows?**  
Start Docker Desktop, then `compose up` again. Restart host Go/Node if you use development mode.

**Where are logs stored?**  
ClickHouse `pulselog.logs`. Identity in PostgreSQL. In-flight ingest in Kafka.

**Will `docker compose down` delete logs?**  
No. Volumes remain unless you pass `-v`.

**What does `down -v` do?**  
Deletes named volumes and **all** local Kafka/CH/PG/Redis data.

**Why is Kafka needed?**  
So ingest HTTP does not wait on ClickHouse and so the processor can catch up after downtime.

**Why ClickHouse?**  
Analytics and time-range scans on log volume.

**Why PostgreSQL and ClickHouse?**  
Different jobs: configuration vs log archive.

**What happens if Redis goes down?**  
Query/ingest `/readyz` fail. Historical ClickHouse data remains. Live frames and rate-limit/denylist/tickets degrade. Rate-limit INCR errors fail **open**. Processor stays ready and keeps writing ClickHouse.

**What happens if the processor stops?**  
Ingest can still 202 (Kafka buffers). Lag grows. Restart drains the topic.

**Can PulseLog lose realtime messages?**  
Yes. Full hub buffer, Redis drop, disconnect. History is still in ClickHouse.

**Are historical logs lost if WebSocket disconnects?**  
No.

**How do I create an API key?**  
Sign in as owner/admin → API Keys → create → copy once.

**Why did I get 403?**  
Key service/project mismatch, signups disabled, or RBAC (for example creating a project as non-owner).

**Why did I get 429?**  
Rate limit. Defaults 120/min ingest per key, 10/min login per IP, 120/min query per IP.

**Why did I get 401?**  
Bad/missing key or JWT.

**Can PulseLog be deployed publicly?**  
Yes, with TLS, closed signups, secrets, and private data stores. Follow DEPLOYMENT.md.

**Is Kubernetes required?**  
No. Compose is enough for demo. Manifests are optional.

**Can it run on one server?**  
Yes (portfolio Compose). That is not HA.

**Which ingest path should I use?**  
Prefer `POST /v1/logs`. `/ingest` is an alias.

**Is there an SDK?**  
No.

**How do I seed the dashboard?**  
`.\scripts\seed-dashboard.ps1` with signups enabled.

**How long are logs kept?**  
90 days raw by default (`CLICKHOUSE_TTL_DAYS`).

**Can I use OFFSET pagination?**  
The API uses keyset cursors. Do not add large OFFSET.

**Does Grafana replace the dashboard?**  
No. Grafana is ops; Next.js is the product.

---

# 75. Command reference

### Docker (local infra)

```powershell
docker compose -f infrastructure/docker-compose.yml up -d
docker compose -f infrastructure/docker-compose.yml ps
docker compose -f infrastructure/docker-compose.yml logs -f --tail=100
docker compose -f infrastructure/docker-compose.yml stop
docker compose -f infrastructure/docker-compose.yml down
# WARNING: deletes volumes
docker compose -f infrastructure/docker-compose.yml down -v
docker compose -f infrastructure/docker-compose.yml --profile app up -d --build
docker compose -f infrastructure/docker-compose.yml --profile obs up -d
```

### Production Compose

```powershell
copy .env.prod.example .env.prod
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml up -d --build
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml ps
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml down
make up-prod
```

### Development

```powershell
go run ./services/ingestion-api
go run ./services/log-processor
$env:CORS_ORIGINS="http://127.0.0.1:3000,http://localhost:3000"; go run ./services/query-api
npm --prefix apps/dashboard run dev
.\scripts\seed-dashboard.ps1
```

### Make

```powershell
make up
make down
make test
make run-ingest
make run-processor
make run-query
make run-dashboard
make fmt
make vet
make images
make up-app
make up-obs
make load-setup
make load-test
make load-query
make load-mixed
```

### Testing

```powershell
go test ./internal/... ./services/...
go test ./internal/... ./services/... ./cmd/...
cd apps/dashboard; npm ci; npm run lint; npm run typecheck; npm test; npm run build
```

### Benchmarking

```powershell
.\scripts\load\setup.ps1
.\scripts\load\ingest.ps1 -Rate 100 -Duration 20s
.\scripts\load\query.ps1
.\scripts\load\mixed.ps1
.\scripts\load\ws.ps1 -Clients 1
.\scripts\load\rate-limit.ps1
```

### Health

```powershell
curl.exe -sS http://127.0.0.1:8080/healthz
curl.exe -sS http://127.0.0.1:8080/readyz
curl.exe -sS http://127.0.0.1:8081/readyz
curl.exe -sS http://127.0.0.1:8082/readyz
```

### Logs / ClickHouse peek (local Compose credentials only)

```powershell
docker exec pulselog-clickhouse-1 clickhouse-client --user pulselog --password pulselog_dev_only --query "SELECT event_id, service, level, message FROM pulselog.logs ORDER BY ingested_at DESC LIMIT 5"
```

### Build images

See [§66](#66-rebuilding-containers).

### Observability

```powershell
make up-obs
# Prometheus http://127.0.0.1:9090
# Grafana    http://127.0.0.1:3001
```

### Kubernetes

```bash
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl apply -k deploy/kubernetes
docker run --rm -v ${PWD}:/src -w /src registry.k8s.io/kustomize/kustomize:v5.4.3 build deploy/kubernetes
```

---

# 76. Glossary

| Term | Simple definition |
| --- | --- |
| **API** | HTTP interface (ingest and query). |
| **API key** | Secret `pl_live_...` for applications to ingest. |
| **JWT** | Signed token proving a user logged in; stored in an httpOnly cookie on the dashboard. |
| **Kafka** | Distributed log/queue. PulseLog uses it as an ingest buffer. |
| **Broker** | A Kafka server process. |
| **Topic** | Named Kafka stream (`logs-ingest`, `logs-dlq`). |
| **Consumer** | Process that reads a topic (the processor). |
| **Consumer group** | Coordinated consumers sharing partitions (`log-processor`). |
| **Offset** | Position in a partition. Committed after successful processing. |
| **Lag** | Messages produced but not yet consumed. |
| **DLQ** | Dead-letter queue (`logs-dlq`) for poison or exhausted writes. |
| **ClickHouse** | Column database for logs and charts. |
| **Redis** | In-memory store used here for counters, denylist, tickets, pub/sub. |
| **Pub/Sub** | Publish a message to a channel; subscribers receive it. |
| **WebSocket** | Long-lived browser connection for live frames. |
| **RBAC** | Role-based access control (owner/admin/member/viewer). |
| **Docker** | Packaged Linux processes (containers) from images. |
| **Container** | Running instance of an image. |
| **Image** | Build artifact. |
| **Volume** | Persistent Docker disk. |
| **Compose** | Multi-container YAML runner. |
| **Health check** | Probe that a process or container is alive (`/healthz`). |
| **Readiness** | Probe that dependencies are reachable (`/readyz`). |
| **CI/CD** | Automated test (CI) and image publish (CD) on GitHub Actions. |
| **Kubernetes** | Cluster orchestrator; this repo has app manifests. |
| **Ingress** | HTTP/S routing into the cluster (dashboard/api/ingest hosts). |
| **Prometheus** | Metric scraper and TSDB. |
| **Grafana** | Ops charting UI. |
| **p50 / p95 / p99** | Latency: 50% / 95% / 99% of requests were faster than this. |
| **BFF** | Backend-for-frontend: Next.js routes that talk to query-api with the cookie. |
| **At-least-once** | A message may be processed more than once; it should not be silently dropped. |

---

# 77. Current project status

**Phases 1–8 are implemented** in this repository ([docs/phases.md](phases.md)).

### Implemented and tested

- Distributed ingest (API keys, validation, Kafka produce)
- Asynchronous processing (batching, retries, DLQ, offset commits)
- ClickHouse analytics storage + TTL + materialized view
- JWT users, argon2id, org/project/service model, RBAC, rate limits
- Query API search, stats, keyset pagination, project isolation
- Next.js dashboard (overview, explorer, management, LIVE)
- Realtime after ClickHouse write (Redis + tickets + WebSocket)
- Prometheus metrics; optional Grafana
- Go unit tests; dashboard Vitest; k6 scenarios; documented local benchmarks
- Production Dockerfiles; production-like Compose; CI; GHCR publish workflow
- Kubernetes application manifests + documented deploy path

### Deployment configuration available

- `infrastructure/docker-compose.prod.yml`
- `deploy/kubernetes/`
- `infrastructure/caddy/Caddyfile`
- [docs/DEPLOYMENT.md](DEPLOYMENT.md)

These are **configuration and packaging**, not a running hosted environment.

### Not currently deployed to paid cloud infrastructure

This repo does **not** create AWS/GCP/Azure resources. No HPA, OpenTelemetry traces, or multi-broker Kafka operations beyond the documented model.

PulseLog as shipped is a complete, testable logging platform you can run locally or on a single VM, with a clear path to managed data stores and Kubernetes for the application tier.
