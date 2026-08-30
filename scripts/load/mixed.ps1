param(
  [int]$Rate = 100,
  [string]$Duration = "30s",
  [int]$QueryVUs = 5,
  [int]$WsClients = 3,
  [string]$RunId = ""
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
if (-not $RunId) { $RunId = New-RunId "mixed" }
$envMap = Get-DefaultLoadEnv $cred $RunId
$envMap.RATE = [string]$Rate
$envMap.DURATION = $Duration
$envMap.QUERY_VUS = [string]$QueryVUs
$envMap.WS_CLIENTS = [string]$WsClients
$envMap.WS_HOLD_MS = "25000"
Assert-ServiceReady "$($envMap.INGEST_URL)/readyz"
Assert-ServiceReady "$($envMap.QUERY_URL)/readyz"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "pre-mixed" -RunId $RunId
$summary = Invoke-K6 -Script "mixed.js" -EnvVars $envMap -SummaryName "mixed-$RunId"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "post-mixed" -RunId $RunId
Write-Output "RUN_ID=$RunId summary=$summary"
