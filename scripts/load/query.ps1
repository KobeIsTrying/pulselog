param(
  [int]$VUs = 8,
  [string]$Duration = "20s",
  [string]$RunId = ""
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
if (-not $RunId) { $RunId = [string]$cred.run_id }
$envMap = Get-DefaultLoadEnv $cred $RunId
$envMap.VUS = [string]$VUs
$envMap.DURATION = $Duration
Assert-ServiceReady "$($envMap.QUERY_URL)/readyz"
$summary = Invoke-K6 -Script "query.js" -EnvVars $envMap -SummaryName "query-$RunId"
Write-Output "RUN_ID=$RunId summary=$summary"
