# Expect 429s when ingest is running with the default 120/min window.
# Do not run this against a process started with RATE_LIMIT_INGEST=1000000.
param(
  [int]$VUs = 8,
  [string]$Duration = "8s"
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
$envMap = Get-DefaultLoadEnv $cred (New-RunId "ratelimit")
$envMap.VUS = [string]$VUs
$envMap.DURATION = $Duration
Assert-ServiceReady "$($envMap.INGEST_URL)/readyz"
$summary = Invoke-K6 -Script "rate-limit.js" -EnvVars $envMap -SummaryName "rate-limit"
Write-Output "summary=$summary"
