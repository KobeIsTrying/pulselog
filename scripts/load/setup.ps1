# Create a dedicated Phase 7 bench identity and write gitignored credentials.
. (Join-Path $PSScriptRoot "common.ps1")

$query = $env:QUERY_URL
if (-not $query) { $query = "http://127.0.0.1:8082" }
$ingest = $env:INGEST_URL
if (-not $ingest) { $ingest = "http://127.0.0.1:8080" }

Assert-ServiceReady "$query/readyz"
Assert-ServiceReady "$ingest/readyz"

$email = $env:PULSELOG_BENCH_EMAIL
if (-not $email) { $email = "bench.phase7@example.com" }
$password = $env:PULSELOG_BENCH_PASSWORD
if (-not $password) { $password = "bench-phase7-pass" }

function Invoke-Json {
  param([string]$Method, [string]$Url, [hashtable]$Headers = @{}, $Body = $null)
  $params = @{ Method = $Method; Uri = $Url; Headers = $Headers }
  if ($null -ne $Body) {
    $params.ContentType = "application/json"
    $params.Body = ($Body | ConvertTo-Json -Compress -Depth 6)
  }
  return Invoke-RestMethod @params
}

$token = $null
try {
  $reg = Invoke-Json POST "$query/api/v1/auth/register" -Body @{
    email      = $email
    password   = $password
    org_name   = "PulseLog Bench"
    org_slug   = "pulselog-bench"
  }
  $token = $reg.token
  Write-Output "registered $email"
} catch {
  $login = Invoke-Json POST "$query/api/v1/auth/login" -Body @{ email = $email; password = $password }
  $token = $login.token
  Write-Output "logged in $email"
}
if (-not $token) { throw "no JWT" }
$auth = @{ Authorization = "Bearer $token" }

$me = Invoke-Json GET "$query/api/v1/auth/me" -Headers $auth
$orgId = $me.orgs[0].org.id
$projects = (Invoke-Json GET "$query/api/v1/orgs/$orgId/projects" -Headers $auth).projects
$project = $projects | Where-Object { $_.slug -eq "default" } | Select-Object -First 1
if (-not $project) { $project = $projects[0] }

foreach ($svc in @("payment-service", "auth-service", "inventory-service", "notification-service", "order-service")) {
  try {
    Invoke-Json POST "$query/api/v1/projects/$($project.id)/services" -Headers $auth -Body @{
      name = $svc
      slug = $svc
    } | Out-Null
  } catch {
    # already exists
  }
}

$keys = [ordered]@{}
foreach ($svc in @("payment-service", "auth-service", "inventory-service", "notification-service", "order-service")) {
  $created = Invoke-Json POST "$query/api/v1/projects/$($project.id)/api-keys" -Headers $auth -Body @{
    name    = "phase7-bench-$svc"
    service = $svc
  }
  if (-not $created.token) { throw "api key token missing for $svc" }
  $keys[$svc] = $created.token
  Write-Output "created key $svc prefix=$($created.prefix)"
}
$key = [pscustomobject]@{ token = $keys["payment-service"] }

$eventId = [guid]::NewGuid().ToString()
$runId = New-RunId "setup"
$body = @{
  event_id  = $eventId
  service   = "payment-service"
  level     = "INFO"
  message   = "bench:$runId setup-pin $eventId"
  timestamp = (Get-Date).ToUniversalTime().ToString("o")
  host      = "bench-setup"
  metadata  = @{ run_id = $runId; source = "setup" }
} | ConvertTo-Json -Compress
$tmp = Join-Path $env:TEMP "pulselog-phase7-setup.json"
[IO.File]::WriteAllText($tmp, $body)
$ing = curl.exe -sS -w "`nHTTP %{http_code}" -X POST "$ingest/v1/logs" -H "Content-Type: application/json" -H "X-API-Key: $($key.token)" --data-binary "@$tmp"
Write-Output "pin ingest $ing"

$cred = [ordered]@{
  email      = $email
  org_id     = $orgId
  project_id = $project.id
  api_key    = $key.token
  api_keys   = $keys
  jwt        = $token
  event_id   = $eventId
  run_id     = $runId
  created_at = (Get-Date).ToUniversalTime().ToString("o")
}
$cred | ConvertTo-Json | Set-Content -Encoding utf8 $CredFile
Write-Output "wrote $CredFile project=$($project.id)"
