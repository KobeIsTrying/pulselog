# Record the local development machine used for a benchmark run.
. (Join-Path $PSScriptRoot "common.ps1")

if (-not (Test-Path $ResultsDir)) {
  New-Item -ItemType Directory -Path $ResultsDir | Out-Null
}

$os = Get-CimInstance Win32_OperatingSystem
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$cs = Get-CimInstance Win32_ComputerSystem
$dockerInfo = docker info --format "{{.OSType}} cpus={{.NCPU}} mem={{.MemTotal}}" 2>&1
$compose = docker compose -f (Join-Path $RepoRoot "infrastructure\docker-compose.yml") ps --format json 2>&1

$info = [ordered]@{
  collected_at_utc   = (Get-Date).ToUniversalTime().ToString("o")
  note               = "Local development machine. Not production capacity."
  os_caption         = $os.Caption
  os_version         = $os.Version
  cs_name            = $cs.Name
  cpu_name           = $cpu.Name
  cpu_cores          = $cpu.NumberOfCores
  cpu_logical        = $cpu.NumberOfLogicalProcessors
  ram_bytes          = [int64]$os.TotalVisibleMemorySize * 1024
  docker_info        = [string]$dockerInfo
  compose_ps         = @($compose)
}
$path = Join-Path $ResultsDir ("env-" + (Get-Date -Format "yyyyMMdd-HHmmss") + ".json")
$info | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 $path
Write-Output ($info | ConvertTo-Json -Depth 4)
Write-Output "wrote $path"
