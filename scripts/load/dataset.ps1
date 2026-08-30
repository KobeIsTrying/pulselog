# Ingest a target number of synthetic rows for query-size tests.
param(
  [int]$Count = 10000,
  [int]$Rate = 250,
  [string]$RunId = ""
)
. (Join-Path $PSScriptRoot "common.ps1")
$cred = Get-LoadCredentials
if (-not $RunId) { $RunId = New-RunId "dataset$Count" }
$seconds = [Math]::Ceiling($Count / [double]$Rate) + 3
$duration = "{0}s" -f $seconds
$envMap = Get-DefaultLoadEnv $cred $RunId
$envMap.RATE = [string]$Rate
$envMap.DURATION = $duration
$envMap.VUS = "20"
$envMap.MAX_VUS = "80"
Write-Output "seeding ~$Count events at $Rate/s for $duration RUN_ID=$RunId"
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "pre-dataset-$Count" -RunId $RunId
Invoke-K6 -Script "ingest.js" -EnvVars $envMap -SummaryName "dataset-$Count-$RunId" | Out-Null
Start-Sleep -Seconds 3
& (Join-Path $PSScriptRoot "snapshot.ps1") -Label "post-dataset-$Count" -RunId $RunId
Write-Output "RUN_ID=$RunId"
