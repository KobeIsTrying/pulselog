# PulseLog Performance Report

These results were measured on a local development machine and should not
be interpreted as production capacity.

Collected 2026-08-30. Host Go services (ingestion-api, log-processor,
query-api) plus Compose Kafka, ClickHouse, PostgreSQL, Redis. Optional
Prometheus/Grafana used the Compose `obs` profile.

## Test Environment

| Item | Value |
| --- | --- |
| OS | Windows 11 Home Single Language 10.0.26200 |
| Machine | ASUS TUF Gaming F15 (`LAPTOP-D22HL0O6`) |
| CPU | 11th Gen Intel Core i5-11400H @ 2.70 GHz |
| Cores | 6 physical / 12 logical |
| Host RAM | 16 GB (16,905,977,856 bytes) |
| Docker Desktop | Linux VM, 12 CPUs, 8,190,050,304 bytes RAM (~7.6 GiB) |
| k6 | v2.2.0 (`C:\Program Files\k6\k6.exe`) |
| Compose | Kafka 3.9.0, ClickHouse 24.8, PostgreSQL 16, Redis 7 |

**This is a development laptop, not a sized production cluster.** Docker
memory is shared by Kafka (~1 GiB) and ClickHouse (~0.7–1.0 GiB) before
the Go processes.

### Baseline service configuration

Unchanged defaults unless noted:

| Setting | Value |
| --- | --- |
| `PROCESSOR_BATCH_SIZE` | 100 |
| `PROCESSOR_BATCH_TIMEOUT` | 500ms |
| `PROCESSOR_MAX_ATTEMPTS` | 5 |
| `WS_HUB_BUFFER` | 256 (kept after experiment) |
| `RATE_LIMIT_INGEST` / `RATE_LIMIT_QUERY` | **120/min in `.env.example`** |

Throughput tests restarted host ingest/query with
`RATE_LIMIT_INGEST=1000000` and `RATE_LIMIT_QUERY=1000000` so the default
120/min window did not cap the run. Those raised limits are **bench-only
process environment**, not a product default.

Machine snapshot: `tests/load/results/env-20260830-141138.json`.

## Architecture

```
App + API key → POST /v1/logs :8080 → Kafka logs-ingest
                                      → log-processor :8081
                                         → ClickHouse pulselog.logs
                                         → Redis PUBLISH (best-effort)
Query API :8082  → REST + WebSocket
Dashboard :3000  → BFF (httpOnly JWT)
```

Live frames are published only after a successful ClickHouse insert.
Subscriber buffers are bounded; full buffers drop frames.

## Dataset

Synthetic generator: `tests/load/lib.js`.

- Services: payment, auth, inventory, notification, order
- Levels: DEBUG / INFO / WARN / ERROR (INFO weighted)
- Each event: `event_id`, timestamp, host, `metadata.run_id`
- Message prefix `bench:{RUN_ID}` so a run can be queried later

API keys are **per service**. A single key bound to `payment-service`
rejects other `service` fields (403). Load scripts send the matching key.

Approximate ClickHouse size during this session:

| Moment | `count()` |
| --- | --- |
| After 100/250/500/1000/1500 ingest | ~52,084 |
| After mixed + more ingest | ~81,070 |
| After +20k seed | **104,948** |
| End of failure probes | ~106,955 |

## Ingestion Benchmark

`POST /v1/logs` with valid `X-API-Key`. k6 `constant-arrival-rate`.
Duration 20s except 1500/s (12s) and the p99 100/s re-run (15s).

| Target | Attempted | 202 | Failed HTTP | Achieved rps | p50 | p95 | p99 | max | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 100/s (20s) | 2000 | 2000 | 0 | 99.98 | 14.53 ms | 15.80 ms | — | 39.62 ms | First clean baseline |
| 100/s (15s) | 1501 | 1501 | 0 | 99.95 | 14.28 ms | 15.69 ms | **17.65 ms** | 29.17 ms | p99 captured |
| 250/s | 4998 | 4998 | 0 | 249.7 | 13.87 ms | 15.65 ms | 24.19 ms | 113.48 ms | 2 dropped iterations |
| 500/s | 9990 | 9990 | 0 | 499.3 | 12.52 ms | 15.60 ms | 21.83 ms | 103.86 ms | 10 dropped iterations |
| 1000/s | 19714 | 19714 | 0 | 982.2 | 14.09 ms | **116.4 ms** | 180.08 ms | 346.87 ms | VU exhaustion (150) |
| 1500/s (12s) | 14513 | 14513 | 0 | 1196 | 150.09 ms | 241.5 ms | 271.47 ms | 315.71 ms | 3487 dropped iterations |

Status distribution on the clean runs: **100% HTTP 202**. No 5xx.

Local saturation region (not production capacity): around **1000–1200
accepted ingest requests/sec** on this laptop. Error rate stayed 0%; the
limit was latency and k6 VU exhaustion, not Kafka/processor fall-behind.

Reproduce:

```powershell
.\scripts\load\setup.ps1
.\scripts\load\ingest.ps1 -Rate 100 -Duration 20s
.\scripts\load\ingest.ps1 -Rate 250 -Duration 20s
.\scripts\load\ingest.ps1 -Rate 500 -Duration 20s -VUs 30 -MaxVUs 100
.\scripts\load\ingest.ps1 -Rate 1000 -Duration 20s -VUs 40 -MaxVUs 150
```

## Processor Throughput

From `/metrics` after the 100→1500 ingest series (processor process
lifetime at that snapshot):

| Metric | Value |
| --- | --- |
| Consumed | 51,649 |
| Written | 51,649 |
| Retries | 0 (during baseline; see failures) |
| DLQ | 0 (during baseline) |
| Batches | 520 |
| Mean batch size | 51,649 / 520 = **99.3** |
| Mean ClickHouse batch write | 7.589 s / 520 = **14.6 ms** |
| Write p95 (histogram) | most batches ≤ 25 ms (490/520); 519/520 ≤ 50 ms |

Almost every flush hit the **100-event** size trigger, not the 500 ms
timeout. That matches 100–1500 events/sec arrival.

Kafka produce (ingest side, same window): count 51,649; mean
414.98 / 51,649 = **8.0 ms**; 51,587/51,649 under 25 ms.

## Kafka Lag

`kafka-consumer-groups.sh --group log-processor --describe` plus
`pulselog_kafka_consumer_lag`.

| Condition | Lag |
| --- | --- |
| Idle / 100–500/s after the run | 0 on all assigned partitions |
| Mid 250/s while WS tests ran | temporary **62** total (17+26+7+12), then 0 |
| 1000/s and 1500/s after the run | 0 |
| Processor stopped, 250/s × 8s | **2001** buffered (398+776+402+425) |
| Processor restarted | 0 within **~1.6 s**; ClickHouse +2001 rows |

Lag temporarily increases under burst or when the processor is down, then
recovers. It did **not** grow continuously while the processor was
running at these local rates.

## ClickHouse Performance

Insert path is the processor batch write above (~15 ms / ~100 rows ≈
**6,800 rows/s** in that process average; the pipeline kept up with
~1200 ingest/s because batches overlap with Kafka fetch).

Query size (same 8 VUs, 20 s, project-scoped):

| Dataset | HTTP p50 | p95 | p99 | Errors |
| --- | --- | --- | --- | --- |
| ~52k rows | 38.19 ms | 63.71 ms | 88.17 ms | 0 |
| ~105k rows | 44.18 ms | 74.78 ms | 93.49 ms | 0 |

During the 52k query run, `docker stats` showed ClickHouse
**~728% CPU** (multi-core) and **~1.05 GiB** RAM. That is a local
hotspot for analytical endpoints, not a crash.

## Query API Performance

~52k-row run, 8 concurrent VUs, 20 s, 0% errors (345→332 iterations of
the full suite). Per-endpoint k6 trends (ms):

| Endpoint | p50 | p95 | p99 |
| --- | --- | --- | --- |
| Recent logs | 48.28 | 71.63 | 95.51 |
| Service filter | 33.67 | 55.29 | 81.25 |
| Level filter | 42.98 | 62.67 | 91.86 |
| Service + level | 32.49 | 49.41 | 63.19 |
| Message substring | 38.79 | 53.46 | 71.85 |
| Event-ID lookup | 30.50 | 51.82 | 79.76 |
| Overview | 31.52 | 49.79 | 78.40 |
| Time-series | 29.33 | 47.09 | 79.79 |
| Service stats | 32.75 | 55.76 | 90.12 |
| Frequent errors | 39.49 | 65.44 | 93.95 |

```powershell
.\scripts\load\query.ps1 -VUs 8 -Duration 20s
```

## Pagination

Keyset (cursor) pages at ~52k and ~105k:

| Dataset | First page p50 / p95 | Later page p50 / p95 |
| --- | --- | --- |
| ~52k | 48.2 / 69.4 ms | 50.2 / 76.3 ms |
| ~105k | 59.5 / 82.2 ms | 62.3 / 85.5 ms |

Later pages stayed within a few milliseconds of the first page. OFFSET
was not used and was not introduced. Keyset cost is dominated by the
filter + `ORDER BY (service, level, timestamp, event_id)` seek, not by
how many rows were skipped.

## WebSocket Performance

Server metrics (`pulselog_ws_*`, `pulselog_realtime_published_total`)
are the source of truth. Live delivery is best-effort.

During 250/s ingest, k6 clients 1 / 5 / 10 (ticket auth, ~18 s hold):

- Tickets and upgrades: **100%**
- Connects 33, disconnects 32, leftover 1 (dashboard)
- **`pulselog_ws_messages_dropped_total` delta = 0**
- Processor published ~21k events in that window
- Hub `delivered` rose by 51,490 (fanout × subscribers)

k6’s `ws_msgs_received` on the dedicated `ws.js` script under-counted
(hello frames only). The mixed workload (below) received **14,703**
WebSocket messages on 3 sessions in 25 s, which matches ingest ×
subscribers much more closely.

Connections stayed stable; no auth failures.

```powershell
.\scripts\load\ws.ps1 -Clients 1
.\scripts\load\ws.ps1 -Clients 5
.\scripts\load\ws.ps1 -Clients 10
```

## Mixed Workload

200 ingest/s + 5 query VUs + 3 WebSocket clients, 25 s.

| Signal | Result |
| --- | --- |
| Ingest / query / ticket checks | 100% (6527/6527) |
| HTTP (ingest+query combined) p50 / p95 / p99 | 21.31 / 56.09 / 85.60 ms |
| WS messages received (k6) | 14,703 |
| WS sessions | 3 |
| Kafka lag after | 0 |
| ClickHouse rows for the run | 4,987 |

Query p95 in mixed (56 ms overall HTTP, which includes cheap ingest) did
**not** blow up versus the idle query p95 (~64 ms). Ingestion and
dashboard reads coexisted on this machine at 200/s.

```powershell
.\scripts\load\mixed.ps1 -Rate 200 -Duration 25s
```

## WebSocket buffer experiment

Slow subscriber: connect, delay reads 4 s, ingest ~400/s for 3 s
(`scripts/load/ws-slow.ps1`).

| Hub buffer | Delivered Δ | Dropped Δ | Frames read after delay |
| --- | --- | --- | --- |
| **256** (default) | 715 | **479** | 734 |
| **1024** | 1201 | **0** | 1233 |

1024 absorbed this ~1.2k burst; 256 dropped once the per-client channel
filled. Memory per idle client grows with buffer × payload (still
bounded). The dashboard already caps the UI at 200 rows and batches
200 ms.

**Kept default 256.** A larger buffer only moves the drop cliff. It does
not make Redis pub/sub durable. `WS_HUB_BUFFER` remains configurable for
local experiments.

## Frontend stress sanity check

Not a formal browser benchmark.

On `http://127.0.0.1:3000/logs` (viewer session, project `default`):

- Page rendered historical rows and stayed interactive
- LIVE toggled on; **Pause** became **Resume**
- Charts on Overview remained usable (empty staging vs populated default)
- No formal heap measurement; no unbounded-growth claim

## Rate-limiter testing

Default **120 ingest / minute / API key** (Redis fixed window), before
the raised-limit restart.

k6 8 VUs × 8 s (`scripts/load/rate-limit.ps1`):

| | |
| --- | --- |
| Requests | 2248 |
| HTTP 429 counted | 2129 |
| Successful HTTP (202) | 25 |
| Check “202 or 429” | 2154 pass / 94 fail (non-429 errors under the flood, likely resets) |

A single setup `POST /v1/logs` **before** the flood returned **202**.

The limiter key is `rl:ingest:key:{api-key-id}:{window}`. Two ingest
processes sharing `REDIS_ADDR` share the counter. Fail-open if Redis
`INCR` errors (`Allow` returns true). Not replaced.

## Failure / Recovery Tests

Volumes were **not** deleted. `docker compose down -v` was not used.

### ClickHouse stopped 15 s

- Ingest stayed **202** (Kafka still up)
- Processor and Query **503** `/readyz`
- Processor logged `clickhouse insert failed, retrying` attempts 1–4
- Lag 1 on the probe partition; offset not advanced until DLQ/success
- After retries exhausted: `pulselog_processor_events_dlq_total{reason="clickhouse_write_failed"}` = 1
- After start: processor ready, probe 202, historical `count()` intact (104,949 → 104,950)
- Event during the outage is on **DLQ**, not silently dropped from Kafka

### Redis stopped 12 s

- Query `/readyz` **503** (denylist + rate-limit dependency)
- Ingest `/readyz` **503**; handler still **202** (rate limit fail-open)
- Processor `/readyz` stayed ready; ClickHouse writes continued
- `count()` 104,951 → 104,952 (historical data intact)
- After start: query ready; ingest ready after Redis health

### Query API restart

Process killed and restarted (also used for the buffer experiment).
Ingest and processor kept running; ClickHouse row count continued to
grow. Dashboard REST/live dropped until reconnect; `/readyz` returned
after listen. Historical queries worked afterward.

### Processor restart

Stopped processor, ingested **~2001** events (250/s × 8 s). Kafka lag
sum **2001**, ClickHouse unchanged (104,953). Restarted processor: lag
**0 in ~1.6 s**, ClickHouse **106,954** (+2001).

Insert-then-commit crash window still exists. This run did not crash
mid-batch, so **duplicates were not measured**. Delivery remains
**at-least-once**, not exactly-once.

### Kafka restarted (volumes kept)

- Ingest and processor `/readyz` **503**
- Probe during outage: **HTTP 503** `kafka_unavailable`
- Query stayed ready
- `docker compose start kafka` (no `-v`); broker became healthy
- Ingest 202 again; consumer rebalanced; lag returned to 0

## Resource Usage

Approximate, from `docker stats --no-stream` and `Get-Process`.
Treat as order-of-magnitude.

| Process / container | Typical CPU | Typical RSS / MemUsage |
| --- | --- | --- |
| ingestion-api (host) | 41 CPU-sec cumulative over the session | ~62 MiB |
| log-processor (host) | 13 CPU-sec cumulative | ~54 MiB |
| query-api (host) | 6 CPU-sec cumulative | ~54 MiB |
| Kafka | 2–20% idle/light; **80–134%** in bursts | ~970 MiB–1.04 GiB |
| ClickHouse | 3–40% ingest; **~728%** during 8-VU query | ~730 MiB–1.05 GiB |
| PostgreSQL | <3% typical | ~45–85 MiB |
| Redis | <5% | ~5–11 MiB |
| Prometheus / Grafana | <1% | ~26 / ~49 MiB |

Obvious local bottleneck under load: **ingest HTTP + Kafka produce
latency** (VU pile-up at 1000+/s), then **ClickHouse CPU** on concurrent
stats queries. Processor batch writes were not the first limiter.

## Optimizations

Process: baseline → measure → only then change.

1. **Observability (not a throughput change):** `pulselog_kafka_consumer_lag`,
   `pulselog_ws_messages_dropped_total`, `WS_HUB_BUFFER`.
2. **Processor batch 100 / 500 ms:** measured mean batch 99.3 and 14.6 ms
   writes. Did **not** raise batch size. Larger batches would trade
   per-event delay for throughput we did not need; ingest saturated first.
3. **WebSocket buffer 256 vs 1024:** measured (table above). **Kept 256.**

## Before vs After

| Change | Before | After | Decision |
| --- | --- | --- | --- |
| Hub buffer 256 → 1024 (slow subscriber, ~1.2k burst) | 479 drops | 0 drops | Keep **256** default; 1024 only delays drops |
| Batch size 100 → 250 | Mean batch 99.3, write 14.6 ms, lag 0 at 1000/s | not applied | No measured processor backlog to fix |

## Known Limitations

- Default 120/min ingest limit will make un-tuned load tests look like a
  429 storm
- One API key is bound to one service name
- Live path is best-effort; slow subscribers lose frames
- At-least-once can duplicate rows across the insert/commit window
- ClickHouse `count()` and stats cost grow with data; this report stops
  near 100k rows
- k6 `ws.js` frame counts were not reliable under burst; use server
  metrics and `mixed.js`
- Local Docker RAM (~7.6 GiB) and a 6-core laptop cap these numbers

## Interpretation

On this development machine, PulseLog accepted **~500 ingest/s** with
~16 ms p95 and **~1000 ingest/s** with p95 rising above 100 ms, while
the processor stayed caught up. Query p95 stayed under **75 ms** at
~105k rows with 8 concurrent users. That is a **local baseline**, not a
capacity rating.

## Reproduce

```powershell
docker compose -f infrastructure/docker-compose.yml up -d
# restart host Go services with raised RATE_LIMIT_* for throughput tests
.\scripts\load\setup.ps1
.\scripts\load\env-info.ps1
.\scripts\load\ingest.ps1 -Rate 100 -Duration 20s
.\scripts\load\query.ps1
.\scripts\load\mixed.ps1
.\scripts\load\rate-limit.ps1   # only while ingest still uses 120/min
.\scripts\load\failures.ps1 -Target clickhouse
make load-test                  # 100/s ingest shortcut
```

Optional ops UI (not the Next.js product dashboard):

```powershell
docker compose -f infrastructure/docker-compose.yml --profile obs up -d
# Prometheus http://127.0.0.1:9090
# Grafana    http://127.0.0.1:3001  (admin / pulselog_dev_only)
```

## Regression

| Suite | Result |
| --- | --- |
| `go test ./internal/... ./services/...` | pass (2026-08-30) |
| `cd apps/dashboard && npm test` | **50/50** pass |
| k6 ingest/query/mixed/rate-limit/ws | executed; summaries under `tests/load/results/` (gitignored) |
