# Seed PulseLog with multi-service events for the dashboard.
# Requires: query-api :8082, ingestion-api :8080, processor writing to ClickHouse.
param(
  [string]$Email = "dashboard.demo@example.com",
  [string]$Password = "dashboard-demo-pass",
  [string]$Organization = "PulseLog Demo"
)

$ErrorActionPreference = "Stop"
$query = "http://127.0.0.1:8082"
$ingest = "http://127.0.0.1:8080"

function Invoke-Json {
  param([string]$Method, [string]$Url, [hashtable]$Headers = @{}, $Body = $null)
  $params = @{ Method = $Method; Uri = $Url; Headers = $Headers }
  if ($null -ne $Body) {
    $params.ContentType = "application/json"
    $params.Body = ($Body | ConvertTo-Json -Compress -Depth 6)
  }
  return Invoke-RestMethod @params
}

$reg = $null
try {
  $reg = Invoke-Json POST "$query/api/v1/auth/register" -Body @{
    email        = $Email
    password     = $Password
    organization = $Organization
  }
} catch {
  $reg = Invoke-Json POST "$query/api/v1/auth/login" -Body @{ email = $Email; password = $Password }
}

$token = $reg.token
$projectId = $reg.project.id
if (-not $projectId) {
  $me = Invoke-Json GET "$query/api/v1/auth/me" -Headers @{ Authorization = "Bearer $token" }
  $orgId = $me.orgs[0].org.id
  $projects = Invoke-Json GET "$query/api/v1/orgs/$orgId/projects" -Headers @{ Authorization = "Bearer $token" }
  $projectId = $projects.projects[0].id
}

$auth = @{ Authorization = "Bearer $token" }
$services = @("payment-service", "auth-service", "inventory-service")
foreach ($name in $services) {
  try { Invoke-Json POST "$query/api/v1/projects/$projectId/services" -Headers $auth -Body @{ name = $name } | Out-Null } catch { }
}

$now = (Get-Date).ToUniversalTime().ToString("o")
$payloads = @(
  @{ service = "payment-service"; level = "INFO"; message = "Payment captured for order ord-phase5" },
  @{ service = "payment-service"; level = "WARN"; message = "Payment retry scheduled for order ord-phase5" },
  @{ service = "payment-service"; level = "ERROR"; message = "Payment authorization failed for order ord-phase5" },
  @{ service = "auth-service"; level = "INFO"; message = "Session issued for dashboard demo user" },
  @{ service = "auth-service"; level = "WARN"; message = "Refresh token reused for dashboard demo user" },
  @{ service = "auth-service"; level = "ERROR"; message = "Login rate limit exceeded for dashboard demo user" },
  @{ service = "inventory-service"; level = "INFO"; message = "Stock reserved for sku sku-phase5" },
  @{ service = "inventory-service"; level = "WARN"; message = "Low stock for sku sku-phase5" },
  @{ service = "inventory-service"; level = "ERROR"; message = "Inventory write conflict for sku sku-phase5" }
)

foreach ($svc in $services) {
  $key = Invoke-Json POST "$query/api/v1/projects/$projectId/api-keys" -Headers $auth -Body @{
    name    = "seed-$svc"
    service = $svc
  }
  $events = $payloads | Where-Object { $_.service -eq $svc }
  foreach ($ev in $events) {
    $body = @{
      service   = $ev.service
      level     = $ev.level
      message   = $ev.message
      timestamp = $now
      host      = "seed-host"
      metadata  = @{ seed = "phase5" }
    }
    Invoke-WebRequest -Method POST -Uri "$ingest/v1/logs" -Headers @{ "X-API-Key" = $key.token } -ContentType "application/json" -Body ($body | ConvertTo-Json -Compress -Depth 5) | Out-Null
  }
}

Write-Host "Seeded $Email / $Password"
Write-Host "Open http://127.0.0.1:3000/login and search for ord-phase5"
