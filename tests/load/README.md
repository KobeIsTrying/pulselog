# PulseLog load tests

k6 scenarios for Phase 7. Credentials are never hardcoded: run
`scripts/load/setup.ps1` first. That writes `tests/load/.credentials.json`
(gitignored).

## Prerequisites

- Compose infra is up (`make up`)
- Host Go services are listening on `:8080`, `:8081`, `:8082`
- For ingestion/query throughput tests, restart ingest/query with raised
  Redis limits so the 120/min default does not cap the run:

```powershell
$env:RATE_LIMIT_INGEST="1000000"
$env:RATE_LIMIT_QUERY="1000000"
$env:CORS_ORIGINS="http://127.0.0.1:3000,http://localhost:3000"
```

Keep `.env.example` defaults at 120/min. Raised limits are a bench-only
process environment.

k6 is resolved as a native `k6` binary, or `docker run grafana/k6`.

## Reproduce

```powershell
.\scripts\load\setup.ps1
.\scripts\load\env-info.ps1
.\scripts\load\ingest.ps1 -Rate 100 -Duration 20s
.\scripts\load\ingest.ps1 -Rate 250 -Duration 20s
.\scripts\load\ingest.ps1 -Rate 500 -Duration 20s
.\scripts\load\query.ps1
.\scripts\load\ws.ps1 -Clients 1
.\scripts\load\mixed.ps1
.\scripts\load\rate-limit.ps1
```

Or `make load-test` (default 100 events/sec ingest for 20s).

Each event includes `metadata.run_id` and `message` prefix `bench:{RUN_ID}`
so ClickHouse rows from a run can be queried afterward.

## Environment variables

| Variable | Default | Used by |
| --- | --- | --- |
| `INGEST_URL` | `http://127.0.0.1:8080` | ingest, mixed, rate-limit |
| `QUERY_URL` | `http://127.0.0.1:8082` | query, mixed, ws |
| `QUERY_WS_URL` | `ws://127.0.0.1:8082` | ws, mixed |
| `PULSELOG_API_KEY` | from credentials file | ingest |
| `PULSELOG_JWT` | from credentials file | query, ws |
| `PULSELOG_PROJECT_ID` | from credentials file | query, ws |
| `RUN_ID` | generated per script | all |
| `RATE` | `100` | ingest, mixed |
| `DURATION` | `20s` / `30s` | all |
| `VUS` / `MAX_VUS` | `20` / `80` | ingest |
| `WS_CLIENTS` | `1` | ws |
| `EVENT_ID` | optional | query pin lookup |
