# PulseLog Compose stack

`docker-compose.yml` is **local development** (infra ports published for host-run binaries).  
`docker-compose.prod.yml` is **production-like** (built app images, migrate job, internal data network).

Passwords in `.env.example` are not for production. Use `.env.prod.example` → `.env.prod`.

```bash
docker compose -f infrastructure/docker-compose.yml up -d
docker compose -f infrastructure/docker-compose.yml ps
docker compose -f infrastructure/docker-compose.yml down -v
```

| Service | Host ports | Role |
| --- | --- | --- |
| kafka | 9092 | KRaft broker+controller; data in named volume `kafka_data` at `/var/lib/kafka/data` (uid 1000 via `kafka-data-init`) |
| kafka-data-init | — | `chown 1000:1000` on `kafka_data`, then exits |
| kafka-init | — | Creates `logs-ingest` and `logs-dlq` then exits |
| clickhouse | 8123 (HTTP), 9000 (native) | Log store and analytics |
| postgres | 5432 | Users / metadata (Phase 3+) |
| redis | 6379 | Cache and live fanout (Phase 4+) |
| prometheus (`obs`) | 9090 | Scrapes host `:8080` `:8081` `:8082` |
| grafana (`obs`) | 3001 | Provisioned PulseLog operations dashboard |

```bash
docker compose -f infrastructure/docker-compose.yml --profile obs up -d
```

Do not use `docker compose down -v` when you need to keep Kafka/ClickHouse data.

Ingestion-api and log-processor are run on the host during local development:

```bash
go run ./services/ingestion-api
go run ./services/log-processor
go run ./services/query-api
```

Ingest uses `localhost:9092`; the processor uses `localhost:9092` and ClickHouse `localhost:9000`. To run them as containers:

```bash
docker compose -f infrastructure/docker-compose.yml --profile app up -d --build
```

That profile uses `KAFKA_BROKERS=kafka:19092` and `CLICKHOUSE_ADDR=clickhouse:9000`. Processor health is on host port **8081**. Query API is on **8082**. Dashboard (standalone) is on **3000**.

Production-like stack (no host-published Postgres/Redis/Kafka/ClickHouse):

```bash
docker compose --env-file ../.env.prod -f docker-compose.prod.yml up -d --build
```

Full deploy notes: [docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md).
