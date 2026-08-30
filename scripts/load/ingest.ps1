param(
  [int]$Rate = 100,
  [string]$Duration = "20s",
  [int]$VUs = 20,
  [int]$MaxVUs = 80,
  [string]$RunId = ""
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
if (-not $RunId) { $RunId = New-RunId "ingest$Rate" }
$envMap = Get-DefaultLoadEnv $cred $RunId
$envMap.RATE = [string]$Rate
$envMap.DURATION = $Duration
$envMap.VUS = [string]$VUs
$envMap.MAX_VUS = [string]$MaxVUs
Assert-ServiceReady "$($envMap.INGEST_URL)/readyz"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "pre-ingest-$Rate" -RunId $RunId
$summary = Invoke-K6 -Script "ingest.js" -EnvVars $envMap -SummaryName "ingest-$Rate-$RunId"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "post-ingest-$Rate" -RunId $RunId
Write-Output "RUN_ID=$RunId summary=$summary"
