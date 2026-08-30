# Slow-subscriber experiment: connect, delay reads, ingest a burst, then drain.
param(
  [int]$Burst = 800,
  [int]$DelayMs = 4000,
  [string]$Label = "ws-slow-256"
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
$query = $env:QUERY_URL
if (-not $query) { $query = "http://127.0.0.1:8082" }
$ingest = $env:INGEST_URL
if (-not $ingest) { $ingest = "http://127.0.0.1:8080" }

function Metric([string]$Name) {
  $t = (Invoke-WebRequest -UseBasicParsing "http://127.0.0.1:8082/metrics").Content
  $line = ($t -split "`n") | Where-Object { $_ -match "^$Name(\{|\s)" } | Select-Object -Last 1
  if (-not $line) { return 0 }
  return [double](($line.Trim() -split "\s+")[-1])
}

$beforeDelivered = Metric "pulselog_ws_messages_delivered_total"
$beforeDropped = Metric "pulselog_ws_messages_dropped_total"

$ticket = (Invoke-RestMethod -Method POST -Uri "$query/api/v1/stream/ticket" -Headers @{ Authorization = "Bearer $($cred.jwt)" }).ticket
$ws = [System.Net.WebSockets.ClientWebSocket]::new()
$uri = [Uri]"ws://127.0.0.1:8082/api/v1/stream?ticket=$ticket&project_id=$($cred.project_id)"
[void]$ws.ConnectAsync($uri, [Threading.CancellationToken]::None).GetAwaiter().GetResult()
Write-Output "connected; delaying reads ${DelayMs}ms while ingesting $Burst"

$runId = New-RunId $Label
$envMap = Get-DefaultLoadEnv $cred $runId
$envMap.RATE = "400"
$envMap.DURATION = "3s"
$envMap.VUS = "20"
$envMap.MAX_VUS = "40"
Invoke-K6 -Script "ingest.js" -EnvVars $envMap -SummaryName "$Label-burst" | Out-Null
Start-Sleep -Milliseconds $DelayMs

$received = 0
$buf = [byte[]]::new(65536)
$deadline = (Get-Date).AddSeconds(8)
while ((Get-Date) -lt $deadline) {
  $cts = [System.Threading.CancellationTokenSource]::new(400)
  try {
    $seg = [ArraySegment[byte]]::new($buf)
    $task = $ws.ReceiveAsync($seg, $cts.Token)
    if (-not $task.Wait(450)) { break }
    if ($task.Result.Count -gt 0) { $received++ }
  } catch {
    break
  } finally {
    $cts.Dispose()
  }
}
$ws.Abort()
$afterDelivered = Metric "pulselog_ws_messages_delivered_total"
$afterDropped = Metric "pulselog_ws_messages_dropped_total"
$deltaD = [int]($afterDelivered - $beforeDelivered)
$deltaX = [int]($afterDropped - $beforeDropped)
Write-Output "received_after_delay=$received delivered_delta=$deltaD dropped_delta=$deltaX burst=$Burst"
$out = [ordered]@{
  label = $Label
  burst = $Burst
  received_after_delay = $received
  delivered_delta = $deltaD
  dropped_delta = $deltaX
}
$path = Join-Path $ResultsDir ("$Label.json")
$out | ConvertTo-Json | Set-Content -Encoding utf8 $path
Write-Output "wrote $path"
