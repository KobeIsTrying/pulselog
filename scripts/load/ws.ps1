param(
  [int]$Clients = 1,
  [string]$Duration = "25s",
  [int]$HoldMs = 20000,
  [string]$RunId = ""
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
if (-not $RunId) { $RunId = New-RunId "ws$Clients" }
$envMap = Get-DefaultLoadEnv $cred $RunId
$envMap.WS_CLIENTS = [string]$Clients
$envMap.DURATION = $Duration
$envMap.WS_HOLD_MS = [string]$HoldMs
Assert-ServiceReady "$($envMap.QUERY_URL)/readyz"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "pre-ws-$Clients" -RunId $RunId
$summary = Invoke-K6 -Script "ws.js" -EnvVars $envMap -SummaryName "ws-$Clients-$RunId"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "post-ws-$Clients" -RunId $RunId
Write-Output "RUN_ID=$RunId summary=$summary"
