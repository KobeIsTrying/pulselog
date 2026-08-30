# Environment variables

No secrets are hardcoded in application code. Copy `.env.example` to `.env` for local Compose and host-run binaries.

## Runtime

| Variable | Default | Used by |
| --- | --- | --- |
| `ENV` | `development` | all Go services |
| `LOG_LEVEL` | `info` | all Go services (`debug`, `info`, `warn`, `error`) |

## Ingestion / HTTP

| Variable | Default | Used by |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | ingestion-api |
| `PROCESSOR_HTTP_ADDR` | `:8081` | log-processor |
| `QUERY_HTTP_ADDR` | `:8082` | query-api |
| `QUERY_TIMEOUT` | `5s` | query-api |
| `WS_HUB_BUFFER` | `256` | query-api (per-subscriber live buffer; not unbounded) |
| `HTTP_MAX_BODY_BYTES` | `5242880` | ingestion-api |

## Kafka

| Variable | Default | Used by |
| --- | --- | --- |
| `KAFKA_BROKERS` | `localhost:9092` | ingestion-api, log-processor |
| `KAFKA_TOPIC_INGEST` | `logs-ingest` | ingestion-api, log-processor |
| `KAFKA_TOPIC_DLQ` | `logs-dlq` | log-processor |
| `KAFKA_CONSUMER_GROUP` | `log-processor` | log-processor |
| `KAFKA_WRITE_TIMEOUT` | `5s` | ingestion-api |

## Processor

| Variable | Default | Used by |
| --- | --- | --- |
| `PROCESSOR_BATCH_SIZE` | `100` | log-processor |
| `PROCESSOR_BATCH_TIMEOUT` | `500ms` | log-processor |
| `PROCESSOR_MAX_ATTEMPTS` | `5` | log-processor |
| `PROCESSOR_RETRY_BACKOFF` | `200ms` | log-processor |
| `PROCESSOR_SHUTDOWN_GRACE` | `15s` | log-processor |
| `CLICKHOUSE_WRITE_TIMEOUT` | `10s` | log-processor |
| `CLICKHOUSE_TABLE` | `logs` | log-processor |

From a container on the Compose network, set `KAFKA_BROKERS=kafka:19092`. From the host, use `localhost:9092`.

## ClickHouse

| Variable | Default | Used by |
| --- | --- | --- |
| `CLICKHOUSE_ADDR` | `localhost:9000` | log-processor, query-api |
| `CLICKHOUSE_DATABASE` | `pulselog` | log-processor, query-api |
| `CLICKHOUSE_USER` | `pulselog` | log-processor, query-api |
| `CLICKHOUSE_PASSWORD` | *(required in non-dev)* | log-processor, query-api, migrate |
| `CLICKHOUSE_TTL_DAYS` | `90` | migrate, log-processor, query-api |
| `CLICKHOUSE_MV_TTL_DAYS` | `180` | migrate, log-processor, query-api |

## PostgreSQL

| Variable | Default | Used by |
| --- | --- | --- |
| `POSTGRES_DSN` | see `.env.example` | ingestion-api, query-api |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | local defaults | Compose only |

## Redis

| Variable | Default | Used by |
| --- | --- | --- |
| `REDIS_ADDR` | `localhost:6379` | ingestion-api, query-api, log-processor (live publish) |

## Auth

| Variable | Default | Used by |
| --- | --- | --- |
| `JWT_SECRET` | local placeholder in development | query-api |
| `JWT_TTL` | `24h` | query-api |
| `AUTH_SIGNUPS` | `true` unless `ENV=production` | query-api |
| `CORS_ORIGINS` | empty (no wildcard) | ingestion-api, query-api |
| `RATE_LIMIT_LOGIN` | `10` / `1m` | query-api |
| `RATE_LIMIT_INGEST` | `120` / `1m` | ingestion-api |
| `RATE_LIMIT_QUERY` | `120` / `1m` | query-api |

## Dashboard (Next.js)

| Variable | Default | Used by |
| --- | --- | --- |
| `QUERY_API_URL` | `http://127.0.0.1:8082` | `apps/dashboard` server routes only |
| `QUERY_WS_PUBLIC_URL` | `ws://127.0.0.1:8082/api/v1/stream` | Dashboard `GET /api/runtime` (container / production) |
| `NEXT_PUBLIC_QUERY_WS_URL` | same | Local `npm run dev` fallback |
| `COOKIE_SECURE` | `false` unless `NODE_ENV=production` | Dashboard session cookie |

Do not expose `JWT_SECRET` or raw API keys through `NEXT_PUBLIC_*` variables.

`JWT_SECRET` and database passwords in `.env.example` are for **local development only**. Production-like Compose uses `.env.prod.example` → `.env.prod`.

Migrations: `cmd/migrate` is the dedicated job. `internal/postgres/migrations/*.sql` are also applied on ingestion-api and query-api startup under `pg_advisory_lock` (`schema_migrations`). ClickHouse schema/TTL is applied by migrate and by processor / query-api `EnsureSchema` (`CREATE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `MODIFY TTL`). Kafka topics `logs-ingest` and `logs-dlq` are created by `kafka-init` with auto-create disabled.

See [docs/DEPLOYMENT.md](DEPLOYMENT.md) for secrets, networks, and TLS.
