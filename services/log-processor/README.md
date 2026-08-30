# log-processor

Consumes `logs-ingest` as consumer group `log-processor`, batch-inserts into ClickHouse `pulselog.logs`, and publishes permanently failed records to `logs-dlq`.

## Run (host)

Kafka and ClickHouse must already be up (`docker compose -f infrastructure/docker-compose.yml up -d`).

```powershell
go run ./services/log-processor
```

Listens on `:8081` by default (`PROCESSOR_HTTP_ADDR`).

- `GET /healthz` — process up
- `GET /readyz` — Kafka TCP + ClickHouse ping
- `GET /metrics` — Prometheus

## Policy

| Case | Action |
| --- | --- |
| Valid event | Batched INSERT, then commit Kafka offsets |
| Invalid JSON / validation | DLQ immediately (`invalid_json` / `validation_failed`) |
| ClickHouse error | Retry 5 times with backoff, then DLQ (`clickhouse_write_failed`) |
| DLQ publish failure | Do not commit offsets |

Delivery is at-least-once. See the root README for schema and duplicate-event notes.
