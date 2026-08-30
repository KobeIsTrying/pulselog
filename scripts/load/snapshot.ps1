# Capture metrics, Kafka lag, ClickHouse counts, and docker stats.
param(
  [string]$Label = "snapshot",
  [string]$RunId = ""
)
. (Join-Path $PSScriptRoot "common.ps1")

if (-not (Test-Path $ResultsDir)) {
  New-Item -ItemType Directory -Path $ResultsDir | Out-Null
}

$ingestM = Get-PrometheusText "http://127.0.0.1:8080/metrics"
$procM = Get-PrometheusText "http://127.0.0.1:8081/metrics"
$queryM = Get-PrometheusText "http://127.0.0.1:8082/metrics"
$lag = Get-KafkaLag
$chTotal = Get-ClickHouseCount
$chRun = $null
if ($RunId) {
  $safe = $RunId.Replace("'", "")
  $chRun = Get-ClickHouseCount "message LIKE 'bench:$safe%'"
}
$stats = docker stats --no-stream --format "{{.Name}} cpu={{.CPUPerc}} mem={{.MemUsage}}" 2>&1

$names = @(
  "pulselog_ingest_events_accepted_total",
  "pulselog_ingest_events_rejected_total",
  "pulselog_processor_events_consumed_total",
  "pulselog_processor_events_written_total",
  "pulselog_processor_events_retried_total",
  "pulselog_kafka_consumer_lag",
  "pulselog_realtime_published_total",
  "pulselog_ws_connections",
  "pulselog_ws_messages_delivered_total",
  "pulselog_ws_messages_dropped_total",
  "pulselog_rate_limited_total"
)

$extracted = [ordered]@{}
foreach ($n in $names) {
  $extracted[$n] = @{
    ingest = Get-MetricValue $ingestM $n
    processor = Get-MetricValue $procM $n
    query = Get-MetricValue $queryM $n
  }
}

$payload = [ordered]@{
  label            = $Label
  collected_at_utc = (Get-Date).ToUniversalTime().ToString("o")
  run_id           = $RunId
  clickhouse_total = $chTotal
  clickhouse_run   = $chRun
  kafka_lag        = $lag
  docker_stats     = @($stats)
  metrics          = $extracted
}
$path = Join-Path $ResultsDir ("snap-" + $Label + "-" + (Get-Date -Format "yyyyMMdd-HHmmss") + ".json")
$payload | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 $path
Write-Output "=== $Label ==="
Write-Output "clickhouse_total=$chTotal run_rows=$chRun"
Write-Output $lag
Write-Output ($stats | Out-String)
Write-Output "wrote $path"
