$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$LoadDir = Join-Path $RepoRoot "tests\load"
$CredFile = Join-Path $LoadDir ".credentials.json"
$ResultsDir = Join-Path $LoadDir "results"

function Assert-ServiceReady {
  param([string]$Url)
  $r = Invoke-RestMethod $Url
  if ($r.status -ne "ok" -and $r.status -ne "ready") {
    throw "$Url not ready: $($r | ConvertTo-Json -Compress)"
  }
}

function Get-LoadCredentials {
  if (-not (Test-Path $CredFile)) {
    throw "Missing $CredFile. Run scripts/load/setup.ps1 first."
  }
  return Get-Content -Raw $CredFile | ConvertFrom-Json
}

function New-RunId {
  param([string]$Prefix = "bench")
  return ("{0}-{1}" -f $Prefix, [guid]::NewGuid().ToString("N").Substring(0, 12))
}

function Resolve-K6 {
  $candidates = @(
    (Get-Command k6 -ErrorAction SilentlyContinue).Source,
    "$env:ProgramFiles\k6\k6.exe",
    "$env:LOCALAPPDATA\Programs\k6\k6.exe"
  ) | Where-Object { $_ -and (Test-Path $_) }
  $native = $candidates | Select-Object -First 1
  if ($native) {
    return [pscustomobject]@{ Kind = "native"; Path = $native }
  }
  $docker = Get-Command docker -ErrorAction SilentlyContinue
  if ($docker) {
    return [pscustomobject]@{ Kind = "docker"; Path = $docker.Source }
  }
  throw "k6 is not installed and docker is unavailable. Install with: winget install GrafanaLabs.k6"
}

function Invoke-K6 {
  param(
    [Parameter(Mandatory = $true)][string]$Script,
    [hashtable]$EnvVars = @{},
    [string]$SummaryName = "",
    [string[]]$ExtraArgs = @()
  )
  if (-not (Test-Path $ResultsDir)) {
    New-Item -ItemType Directory -Path $ResultsDir | Out-Null
  }
  $k6 = Resolve-K6
  $scriptPath = Join-Path $LoadDir $Script
  if (-not (Test-Path $scriptPath)) {
    throw "missing k6 script $scriptPath"
  }
  if (-not $SummaryName) {
    $SummaryName = [IO.Path]::GetFileNameWithoutExtension($Script) + "-" + (Get-Date -Format "yyyyMMdd-HHmmss")
  }
  $summaryHost = Join-Path $ResultsDir ($SummaryName + ".json")

  Write-Output "k6 runner=$($k6.Kind) script=$Script summary=$summaryHost"
  foreach ($k in ($EnvVars.Keys | Sort-Object)) {
    if ($k -match "KEY|JWT|TOKEN|PASSWORD|SECRET") {
      Write-Output "  $k=<redacted>"
    } else {
      Write-Output "  $k=$($EnvVars[$k])"
    }
  }

  if ($k6.Kind -eq "native") {
    foreach ($k in $EnvVars.Keys) {
      Set-Item -Path "Env:$k" -Value ([string]$EnvVars[$k])
    }
    & $k6.Path run --summary-export $summaryHost @ExtraArgs $scriptPath
    $code = $LASTEXITCODE
  } else {
    $dockerArgs = @(
      "run", "--rm",
      "--add-host=host.docker.internal:host-gateway",
      "-v", "${LoadDir}:/scripts",
      "-e", "INGEST_URL=$($EnvVars['INGEST_URL'])",
      "-e", "QUERY_URL=$($EnvVars['QUERY_URL'])",
      "-e", "QUERY_WS_URL=$($EnvVars['QUERY_WS_URL'])"
    )
    foreach ($k in $EnvVars.Keys) {
      $dockerArgs += @("-e", "$k=$($EnvVars[$k])")
    }
    $dockerArgs += @(
      "grafana/k6:0.54.0", "run",
      "--summary-export", "/scripts/results/$($SummaryName).json"
    )
    $dockerArgs += $ExtraArgs
    $dockerArgs += "/scripts/$Script"
    & docker @dockerArgs
    $code = $LASTEXITCODE
  }
  if ($code -ne 0) {
    Write-Warning "k6 exited $code (thresholds may have failed; summary still written if present)"
  }
  if (Test-Path $summaryHost) {
    Write-Output "wrote $summaryHost"
  }
  return $summaryHost
}

function Get-DefaultLoadEnv {
  param($Cred, [string]$RunId)
  $ingest = $env:INGEST_URL
  if (-not $ingest) { $ingest = "http://127.0.0.1:8080" }
  $query = $env:QUERY_URL
  if (-not $query) { $query = "http://127.0.0.1:8082" }
  $ws = $env:QUERY_WS_URL
  if (-not $ws) { $ws = "ws://127.0.0.1:8082" }
  return @{
    INGEST_URL          = $ingest
    QUERY_URL           = $query
    QUERY_WS_URL        = $ws
    PULSELOG_API_KEY    = [string]$Cred.api_key
    PULSELOG_API_KEYS   = ($(if ($Cred.api_keys) { $Cred.api_keys | ConvertTo-Json -Compress } else { "" }))
    PULSELOG_JWT        = [string]$Cred.jwt
    PULSELOG_PROJECT_ID = [string]$Cred.project_id
    RUN_ID              = $RunId
    EVENT_ID            = [string]$Cred.event_id
  }
}

function Get-PrometheusText {
  param([string]$Url)
  try {
    return (Invoke-WebRequest -UseBasicParsing $Url).Content
  } catch {
    return "# unavailable $Url : $($_.Exception.Message)"
  }
}

function Get-MetricValue {
  param([string]$Text, [string]$Name)
  $line = ($Text -split "`n") | Where-Object { $_ -match "^$([regex]::Escape($Name))(\{[^}]*\})?\s+" } | Select-Object -Last 1
  if (-not $line) { return $null }
  $parts = $line.Trim() -split "\s+"
  return $parts[-1]
}

function Get-KafkaLag {
  try {
    $out = docker exec pulselog-kafka-1 /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:19092 --group log-processor --describe 2>&1
    return ($out | Out-String)
  } catch {
    return "kafka lag unavailable: $($_.Exception.Message)"
  }
}

function Get-ClickHouseCount {
  param([string]$Where = "1")
  $q = "SELECT count() FROM pulselog.logs WHERE $Where"
  return (docker exec pulselog-clickhouse-1 clickhouse-client --user pulselog --password pulselog_dev_only --query $q)
}
