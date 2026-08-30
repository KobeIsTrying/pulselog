# query-api

Read-only HTTP API over ClickHouse plus identity/admin APIs. Listens on `:8082`.

Protected log and stats routes require `Authorization: Bearer <jwt>`. Results are scoped to the caller's projects.

Live updates: `POST /api/v1/stream/ticket` then `GET /api/v1/stream?ticket=&project_id=`. Authorization is checked server-side before the WebSocket upgrade. Redis pub/sub (`pulselog:logs:{project_id}`) fans out across instances. REST stays available if the subscriber restarts.

```powershell
go run ./services/query-api
```

PostgreSQL migrations and the ClickHouse `project_id` column are applied on startup. See the root README for auth, RBAC, and rate limits.
