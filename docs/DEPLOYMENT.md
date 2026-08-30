# PulseLog Deployment Guide

This document describes how to run PulseLog as a production-style system. It does not provision paid cloud resources.

**Portfolio/demo deployment, not recommended HA production architecture.** A single Docker Compose VM is suitable for a portfolio demo. A production HA deployment should use managed Kafka, ClickHouse, PostgreSQL, and Redis, plus multiple application replicas behind a TLS-terminating load balancer or ingress.

## Architecture

```
                    TLS terminates at load balancer / ingress / Caddy
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          │                           │                           │
          ▼                           ▼                           ▼
 https://pulselog.example.com   https://api.pulselog.example.com  https://ingest.pulselog.example.com
          │                           │                           │
     dashboard :3000            query-api :8082              ingestion-api :8080
     (Next.js standalone)       REST + WebSocket             POST /v1/logs
          │                           │                           │
          └──────── BFF JWT ──────────┤                           │
                                      │                           ▼
                                      │                     Kafka logs-ingest
                                      │                           │
                                      │                           ▼
                                      │                    log-processor :8081
                                      │                      │           │
                                      │                      ▼           ▼
                                      │                 ClickHouse     logs-dlq
                                      │                      │
                                      └──────── query ───────┘
                                      Redis: rate limits, JWT denylist, live pub/sub
                                      PostgreSQL: identity, API keys, migrations
```

Application services are stateless. Durable state lives in PostgreSQL (identity), ClickHouse (logs), Kafka (in-flight ingest), and Redis (ephemeral limits / denylist / fanout). Kafka is **not** the long-term log archive.

Local development still uses `infrastructure/docker-compose.yml` plus `go run` / `npm run dev`. Production-like runs use `infrastructure/docker-compose.prod.yml` and built images only.

## Container Images

| Image | Dockerfile | User | Port |
| --- | --- | --- | --- |
| `ingestion-api` | `services/ingestion-api/Dockerfile` | 65532 | 8080 |
| `log-processor` | `services/log-processor/Dockerfile` | 65532 | 8081 |
| `query-api` | `services/query-api/Dockerfile` | 65532 | 8082 |
| `dashboard` | `apps/dashboard/Dockerfile` | 1001 | 3000 |
| `migrate` | `cmd/migrate/Dockerfile` | 65532 | — |

Go images are multi-stage, `CGO_ENABLED=0`, Alpine runtime, no source or toolchain in the final layer. The dashboard image is a Next.js `output: "standalone"` production build (`node server.js`), not `npm run dev`.

`.dockerignore` excludes `.env*`, credentials, docs, and the dashboard tree from Go builds. Secrets are injected at runtime.

Build:

```powershell
docker build -f services/ingestion-api/Dockerfile -t pulselog/ingestion-api:local .
docker build -f services/log-processor/Dockerfile -t pulselog/log-processor:local .
docker build -f services/query-api/Dockerfile -t pulselog/query-api:local .
docker build -f cmd/migrate/Dockerfile -t pulselog/migrate:local .
docker build -f apps/dashboard/Dockerfile -t pulselog/dashboard:local apps/dashboard
```

Registry tags (GitHub Container Registry): commit SHA, `main`, and `vMAJOR.MINOR.PATCH`. Do not rely only on `latest`.

## Configuration

Copy `.env.example` for host-run development. Copy `.env.prod.example` to `.env.prod` for Compose production-like runs.

| Kind | Examples |
| --- | --- |
| Required | `KAFKA_BROKERS`, `POSTGRES_DSN`, `CLICKHOUSE_ADDR`, `REDIS_ADDR`, `JWT_SECRET` (query-api / production) |
| Optional | `LOG_LEVEL`, `WS_HUB_BUFFER`, batch sizes, timeouts, `CLICKHOUSE_TTL_DAYS` |
| Development-only | `AUTH_SIGNUPS=true`, localhost CORS fallback, default JWT placeholder |
| Production-only | `ENV=production`, `AUTH_SIGNUPS=false`, `COOKIE_SECURE=true`, explicit `CORS_ORIGINS` |
| Secrets | `JWT_SECRET`, `POSTGRES_DSN` / `POSTGRES_PASSWORD`, `CLICKHOUSE_PASSWORD`, Grafana admin password, cloud credentials |

Rate-limit defaults stay **120/min** ingest and query, **10/min** login. Phase 7 benchmark overrides (`RATE_LIMIT_INGEST=1000000`) must never become production defaults.

## Secrets

Never bake secrets into images or commit them.

Supply them from:

1. Compose: `--env-file .env.prod` (file gitignored)
2. Kubernetes: `Secret` / External Secrets / Sealed Secrets (`deploy/kubernetes/secret.example.yaml` is a template only)
3. Cloud secret manager (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault) mounted as env vars

Do not commit API keys, JWT secrets, passwords, or cloud credentials. Local placeholders (`pulselog_dev_only`, `pulselog_dev_jwt_only`) are development-only.

## Networking

**Public ingress only:**

- Dashboard (HTTPS)
- Query API + WebSocket (HTTPS / WSS)
- Ingestion API (HTTPS)

**Do not publish:**

- PostgreSQL 5432
- Redis 6379
- Kafka broker / controller ports
- ClickHouse 8123 / 9000
- Processor `:8081`
- `/metrics` on a public listener
- Prometheus / Grafana except on localhost or behind auth

Production Compose uses two networks: `pulselog_edge` (published app ports) and `pulselog_data` (`internal: true` for Kafka, ClickHouse, PostgreSQL, Redis, processor, migrate).

## Databases

### PostgreSQL

- Local Compose: `postgres:16-alpine` with a named volume.
- Production recommendation: managed PostgreSQL (RDS, Cloud SQL, Azure Database) with TLS (`sslmode=require`), automated backups, and connection limits sized to replica count.
- Migrations: `internal/postgres/migrations/*.sql` applied by `cmd/migrate` (dedicated job) and again on ingestion-api / query-api startup. `pg_advisory_lock(59204711)` serializes replicas so they cannot race destructively. Failures abort the process.
- Repeatable: already-applied versions in `schema_migrations` are skipped.

### ClickHouse

- Schema bootstrap: `infrastructure/clickhouse/init.sql` on first volume init **and** `EnsureSchema` in migrate / processor / query-api (`CREATE IF NOT EXISTS` + `ADD COLUMN IF NOT EXISTS` + `MODIFY TTL`).
- Upgrades: add new idempotent statements to `EnsureSchema` / init SQL. Do not require manual SQL on a fresh deploy.
- Compatibility: `project_id` is added if missing so pre-auth volumes keep working. Query filters still hide the zero UUID.
- Retention: `CLICKHOUSE_TTL_DAYS` (default 90) and `CLICKHOUSE_MV_TTL_DAYS` (default 180). TTL is applied with `ALTER TABLE ... MODIFY TTL` so existing volumes pick up the configured window.
- Production: persistent disks or ClickHouse Cloud; backups; do not treat local Compose volumes as durability.

### Redis

- Local: AOF enabled. Production: managed Redis.
- Used for rate-limit counters, JWT denylist, and best-effort pub/sub.
- Persistence is not required for correctness of historical logs. Failover drops live frames and may reset windows / denylist entries until TTL expiry.
- Query-api and ingestion-api **readiness fails** if Redis is down (limits + denylist). Processor readiness does **not** require Redis (fanout is best-effort).

## Kafka

Topics are created by `kafka-init` (`--if-not-exists`). Auto-create is **disabled**.

| Topic | Local partitions | Local RF |
| --- | --- | --- |
| `logs-ingest` | 6 | 1 |
| `logs-dlq` | 3 | 1 |

Local single-broker KRaft is **not** production-ready. Production should use a multi-broker cluster (MSK, Confluent Cloud, or self-managed) with:

- replication factor ≥ 3 for ingest and DLQ
- partition count ≥ consumer parallelism you want
- retention sized to the crash/replay window, not as the archive (ClickHouse is the archive)
- TLS and SASL
- broker / consumer-lag monitoring

## TLS

Do not terminate TLS inside every Go service. Terminate at:

- Caddy / nginx / Envoy
- cloud load balancer
- Kubernetes ingress

Internal service traffic on a private network may stay plaintext for a portfolio/demo. Production HA should use mesh or cloud-private TLS to data stores (`sslmode=require` for PostgreSQL, Kafka SSL, ClickHouse secure native).

Local HTTP: `COOKIE_SECURE=false`. HTTPS: `COOKIE_SECURE=true`.

## Reverse Proxy

Example hostnames:

| Host | Upstream | Notes |
| --- | --- | --- |
| `pulselog.example.com` | dashboard:3000 | BFF cookies |
| `api.pulselog.example.com` | query-api:8082 | REST + **WebSocket** `/api/v1/stream` |
| `ingest.pulselog.example.com` | ingestion-api:8080 | API keys |

Caddy (`infrastructure/caddy/Caddyfile`, Compose profile `proxy`) proxies WebSocket upgrades without extra config. Ingress annotations in `deploy/kubernetes/ingress.yaml` raise proxy timeouts for long-lived sockets.

Set `CORS_ORIGINS` and `QUERY_WS_PUBLIC_URL` to those public origins (`wss://api.pulselog.example.com/api/v1/stream`). Production CORS never defaults to `*`.

## CI/CD

GitHub Actions:

- `.github/workflows/ci.yml` — on pull request and `main`: Go fmt/vet/test, dashboard lint/typecheck/test/production build, image builds, Compose + Kubernetes validate, gitleaks (enforced), govulncheck / npm audit / Trivy (advisory).
- `.github/workflows/publish.yml` — after CI, push images to GHCR tagged with SHA, branch (`main`), and semver when a `v*.*.*` tag is pushed.

Do not deploy untested commits. Publish is gated on the CI workflow.

### Enforced vs advisory

| Check | Policy |
| --- | --- |
| gofmt, vet, Go tests | enforced |
| dashboard lint, typecheck, test, `next build` | enforced |
| Docker image build | enforced |
| Compose / kustomize / kubeconform | enforced |
| gitleaks | enforced |
| govulncheck, npm audit, Trivy | advisory (`continue-on-error`) |

## Kubernetes

Manifests live in `deploy/kubernetes/` (Kustomize). They cover **application workloads only**:

- Namespace, ConfigMap, Secret *reference*, migrate Job, Deployments, Services, Ingress
- readiness `/readyz`, liveness `/healthz` (dashboard uses `/login`)
- resource requests/limits, non-root, no privileged containers

Recommended production data plane: managed Kafka, ClickHouse Cloud (or equivalent), managed PostgreSQL, managed Redis. Do not treat in-cluster single-node Kafka/CH/PG/Redis as HA.

```bash
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl -n pulselog apply -f deploy/kubernetes/secret.example.yaml  # replace values first
kubectl apply -k deploy/kubernetes
```

Validate:

```bash
docker run --rm -v ${PWD}:/src -w /src registry.k8s.io/kustomize/kustomize:v5.4.3 build deploy/kubernetes
docker run --rm -v ${PWD}:/src -w /src ghcr.io/yannh/kubeconform:v0.6.7 -strict -ignore-missing-schemas deploy/kubernetes
```

## Portfolio Deployment

**Portfolio/demo deployment, not recommended HA production architecture.**

One VM + Docker Compose + Caddy (or another reverse proxy) + named volumes:

```powershell
copy .env.prod.example .env.prod
# set JWT_SECRET, DB passwords, CORS_ORIGINS, QUERY_WS_PUBLIC_URL
docker compose --env-file .env.prod -f infrastructure/docker-compose.prod.yml up -d --build
```

Optional TLS proxy: `--profile proxy`. Optional metrics: `--profile obs` (Grafana **requires** a real admin password; anonymous auth is off; ports bind to `127.0.0.1`).

Security minimums for a demo VM:

- TLS at the proxy
- firewall: 80/443 (and 22 for you) only
- secrets in `.env.prod` with mode 600, not in git
- volume backups (PostgreSQL + ClickHouse)
- do not publish 5432 / 6379 / 9092 / 9000 / 8081 / 9090 / 3001 to the internet

## Production Deployment

Cloud-neutral path:

1. Build and push images to GHCR (or another registry).
2. Run ingestion-api, log-processor, query-api, dashboard on a container platform (ECS/Fargate, Cloud Run + a worker, or Kubernetes).
3. Point them at managed Kafka, ClickHouse, PostgreSQL, and Redis.
4. Terminate TLS at the load balancer / ingress.
5. Run `migrate` as a one-shot job before rolling new app replicas.
6. Keep `AUTH_SIGNUPS=false` unless you have an invite process.

Concrete example (AWS, not auto-provisioned):

- Dashboard + APIs: ECS Fargate or EKS using the images in this repo
- Kafka: Amazon MSK
- ClickHouse: ClickHouse Cloud
- PostgreSQL: Amazon RDS
- Redis: ElastiCache
- Secrets: AWS Secrets Manager
- Ingress: ALB (HTTP + WebSocket) or an nginx ingress

Do not run the paid provisioners from this repository.

## Scaling

| Service | How it scales | Constraint |
| --- | --- | --- |
| ingestion-api | Stateless horizontal replicas | Kafka produce + Postgres/Redis connections |
| log-processor | One consumer-group member per Kafka partition | 6 local partitions ⇒ max 6 useful replicas; production partition count sets the cap |
| query-api | Horizontal replicas | Redis pub/sub fans out live events; no sticky WS required |
| dashboard | Stateless frontend replicas | BFF calls query-api; cookies are JWT, not server sessions |

Processor replicas beyond the partition count idle. Do not scale the processor past `logs-ingest` partitions.

## Backups

| Store | Expectation |
| --- | --- |
| PostgreSQL | Daily automated backups + PITR if the provider offers it. This is identity/API keys. |
| ClickHouse | Snapshot / backup of `pulselog.logs` and `logs_per_minute`. This is the log archive. |
| Kafka | Not the durable archive. Size retention for replay, not years of logs. |
| Redis | Best-effort. Losing Redis does not delete ClickHouse history. |

Recovery assumption: restore PostgreSQL then ClickHouse; replay leftover Kafka only if you accept at-least-once duplicates (`event_id`).

## Retention

Raw logs TTL defaults to **90 days** (`CLICKHOUSE_TTL_DAYS`). Per-minute rollups default to **180 days**. ClickHouse drops expired parts in the background. Do not silently keep logs forever. Increase or decrease via env; migrate/processor/query apply `MODIFY TTL` on startup.

## Monitoring

Optional Compose profile `obs`:

- Prometheus scrapes application `/metrics` on the Docker network (`prometheus.prod.yml`)
- Grafana is provisioned with the PulseLog ops dashboard
- Local development Compose still scrapes `host.docker.internal` so host-run binaries work

Production:

- Do not expose Grafana with default credentials or anonymous Viewer
- Bind Prometheus/Grafana to localhost or put them behind SSO
- Alert on ingest 5xx, processor lag (`pulselog_kafka_consumer_lag`), ClickHouse write errors, query 5xx, disk usage

## Security

- Non-root images (65532 / 1001)
- No privileged containers in Compose or Kubernetes
- CORS allow-list; `*` rejected; production does not implicit-allow localhost
- Session cookie: httpOnly, SameSite=Lax, Secure when HTTPS / `COOKIE_SECURE=true`
- `AUTH_SIGNUPS=false` in production
- Rate limits default to 120/min, not bench values
- Structured JSON logs to stdout (`service`, `env`, `level`, `request_id`). Do not log raw API keys, passwords, JWTs, or cookie values
- `/healthz` is liveness (process up). `/readyz` is readiness (required deps). Optional live-fanout degradation does not change processor readiness
- Metrics stay off the public edge

## Health / readiness

| Service | Liveness | Readiness requires |
| --- | --- | --- |
| ingestion-api | `/healthz` | Kafka, PostgreSQL, Redis |
| log-processor | `/healthz` | Kafka, ClickHouse (Redis optional) |
| query-api | `/healthz` | ClickHouse, PostgreSQL, Redis |
| dashboard | `/login` | process up (BFF errors if query-api is down) |

Compose `depends_on` waits for healthchecks / `service_completed_successfully` (kafka-init, migrate). That is not a substitute for `/readyz`.

## Graceful shutdown

SIGTERM / SIGINT:

- HTTP servers `Shutdown` with a 15s timeout
- Processor flushes the in-memory batch (`PROCESSOR_SHUTDOWN_GRACE`, default 15s) then closes the Kafka reader
- Query-api cancel stops the Redis subscriber; `http.Server.Shutdown` closes WebSockets
- Compose `stop_grace_period` is 20s (processor 25s)

## Rollback

1. Redeploy the previous image digest (GHCR SHA tag).
2. Schema migrations are additive (`IF NOT EXISTS`). There is no automatic down-migration.
3. If a migration fails, the migrate job / process exits non-zero and replicas that have not started stay off the new code.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| ingest 503 `kafka_unavailable` | Kafka health, `KAFKA_BROKERS` |
| ingest/query 503 not_ready | Postgres / Redis / ClickHouse `/readyz` |
| migrate failed | `docker compose logs migrate` — lock or SQL error |
| dashboard login cookie missing | `COOKIE_SECURE` vs HTTP; SameSite on cross-site HTTPS |
| live WS never connects | `CORS_ORIGINS`, `QUERY_WS_PUBLIC_URL`, ticket endpoint, Redis |
| duplicate `event_id` | at-least-once crash window; query `LIMIT 1 BY event_id` |
| 429 storms | you copied bench rate limits or a key is shared too widely |

## Release process

There is no fabricated release history. When you intend to ship:

1. CI green on `main`
2. Tag `v0.1.0` (then `v0.2.0`, `v1.0.0`, …)
3. Publish workflow pushes `ghcr.io/<org>/pulselog/<service>:<tag>` and the commit SHA
4. Deploy that digest. Roll forward or back by digest, not `latest`
