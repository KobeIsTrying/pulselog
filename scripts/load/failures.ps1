# Controlled local failure/recovery checks. Never uses compose down -v.
param(
  [ValidateSet("clickhouse", "redis", "kafka")]
  [string]$Target = "clickhouse",
  [int]$DownSeconds = 20
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
$ingest = $env:INGEST_URL
if (-not $ingest) { $ingest = "http://127.0.0.1:8080" }
$proc = "http://127.0.0.1:8081"
$query = $env:QUERY_URL
if (-not $query) { $query = "http://127.0.0.1:8082" }

function Get-Ready {
  param([string]$Url)
  try {
    return Invoke-RestMethod $Url
  } catch {
    return @{ status = "error"; error = $_.Exception.Message }
  }
}

function Send-Probe {
  $eventId = [guid]::NewGuid().ToString()
  $body = @{
    event_id  = $eventId
    service   = "payment-service"
    level     = "WARN"
    message   = "bench:failure-$Target $eventId"
    timestamp = (Get-Date).ToUniversalTime().ToString("o")
    host      = "bench-failure"
    metadata  = @{ target = $Target }
  } | ConvertTo-Json -Compress
  $tmp = Join-Path $env:TEMP "pulselog-failure.json"
  [IO.File]::WriteAllText($tmp, $body)
  $out = curl.exe -sS -w "`nHTTP %{http_code}" -X POST "$ingest/v1/logs" -H "Content-Type: application/json" -H "X-API-Key: $($cred.api_key)" --data-binary "@$tmp"
  return [pscustomobject]@{ EventId = $eventId; Response = $out }
}

Write-Output "=== before $Target stop ==="
Write-Output "processor=$(Get-Ready "$proc/readyz" | ConvertTo-Json -Compress)"
Write-Output "query=$(Get-Ready "$query/readyz" | ConvertTo-Json -Compress)"
Write-Output "ingest=$(Get-Ready "$ingest/readyz" | ConvertTo-Json -Compress)"
$before = Send-Probe
Write-Output "probe_before=$($before.Response)"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "fail-pre-$Target"

Write-Output "=== stopping $Target for ${DownSeconds}s (volumes kept) ==="
docker compose -f (Join-Path $RepoRoot "infrastructure\docker-compose.yml") stop $Target
Start-Sleep -Seconds 3
Write-Output "processor=$(Get-Ready "$proc/readyz" | ConvertTo-Json -Compress)"
Write-Output "query=$(Get-Ready "$query/readyz" | ConvertTo-Json -Compress)"
Write-Output "ingest=$(Get-Ready "$ingest/readyz" | ConvertTo-Json -Compress)"
$during = Send-Probe
Write-Output "probe_during=$($during.Response)"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "fail-down-$Target"
Start-Sleep -Seconds $DownSeconds

Write-Output "=== starting $Target ==="
docker compose -f (Join-Path $RepoRoot "infrastructure\docker-compose.yml") start $Target
$deadline = (Get-Date).AddSeconds(90)
do {
  Start-Sleep -Seconds 3
  $st = docker compose -f (Join-Path $RepoRoot "infrastructure\docker-compose.yml") ps $Target --format json 2>&1
  Write-Output "wait $Target $st"
} while ((Get-Date) -lt $deadline -and ($st -notmatch "healthy|running"))

Start-Sleep -Seconds 5
Write-Output "processor=$(Get-Ready "$proc/readyz" | ConvertTo-Json -Compress)"
Write-Output "query=$(Get-Ready "$query/readyz" | ConvertTo-Json -Compress)"
Write-Output "ingest=$(Get-Ready "$ingest/readyz" | ConvertTo-Json -Compress)"
$after = Send-Probe
Write-Output "probe_after=$($after.Response)"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "fail-post-$Target"
Write-Output "done $Target (no volumes deleted)"
