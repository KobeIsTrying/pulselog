# scripts

`seed-dashboard.ps1` registers (or logs in) a demo user, creates payment/auth/inventory
services plus API keys, and sends INFO/WARN/ERROR events through the real ingest path.

```powershell
# after compose + Go services are up
.\scripts\seed-dashboard.ps1
```

Phase 7 load tests live in `scripts/load` and `tests/load`. See
`tests/load/README.md` and [docs/PERFORMANCE.md](../docs/PERFORMANCE.md).

```powershell
.\scripts\load\setup.ps1
.\scripts\load\ingest.ps1 -Rate 100 -Duration 20s
.\scripts\load\query.ps1
.\scripts\load\mixed.ps1
```
