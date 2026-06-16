<#
.SYNOPSIS
    Post-install health check — verifies all services are running and the API is reachable.
.PARAMETER InstallDir
    Root install directory.
.PARAMETER Retries
    Number of times to retry the API endpoint check (default 12, ~60 s).
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$InstallDir,
    [int]$Retries = 12
)

$ErrorActionPreference = "Stop"
$failed = @()

# ── Service status ────────────────────────────────────────────────────────────
$services = @(
    @{ Name = "tpt-postgresql"; Display = "PostgreSQL" },
    @{ Name = "tpt-redis";      Display = "Redis" },
    @{ Name = "tpt-minio";      Display = "MinIO" },
    @{ Name = "tpt-api";        Display = "TPT API" },
    @{ Name = "tpt-worker";     Display = "TPT Worker" },
    @{ Name = "tpt-live";       Display = "TPT Live" }
)

foreach ($svc in $services) {
    $s = Get-Service $svc.Name -ErrorAction SilentlyContinue
    if (-not $s -or $s.Status -ne "Running") {
        $failed += "$($svc.Display) service is not running"
    }
}

# ── API liveness probe ────────────────────────────────────────────────────────
$apiUp = $false
for ($i = 0; $i -lt $Retries; $i++) {
    try {
        $resp = Invoke-WebRequest -Uri "http://localhost:8080/healthz" -UseBasicParsing -TimeoutSec 5
        if ($resp.StatusCode -eq 200) { $apiUp = $true; break }
    } catch { }
    Start-Sleep -Seconds 5
}
if (-not $apiUp) {
    $failed += "API /healthz did not respond with 200 after $($Retries * 5) seconds"
}

# ── API readiness probe ───────────────────────────────────────────────────────
if ($apiUp) {
    try {
        $resp = Invoke-WebRequest -Uri "http://localhost:8080/readyz" -UseBasicParsing -TimeoutSec 10
        if ($resp.StatusCode -ne 200) {
            $failed += "API /readyz returned HTTP $($resp.StatusCode)"
        }
    } catch {
        $failed += "API /readyz check failed: $_"
    }
}

# ── Write health report ───────────────────────────────────────────────────────
$reportPath = Join-Path $InstallDir "logs\install-healthcheck.txt"
$lines  = @("TPT Online Video — install health check $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')")
$lines += ""
foreach ($svc in $services) {
    $s      = Get-Service $svc.Name -ErrorAction SilentlyContinue
    $status = if ($s) { $s.Status } else { "NOT FOUND" }
    $ok     = if ($status -eq "Running") { "OK" } else { "FAIL" }
    $lines += "  [$ok] $($svc.Display): $status"
}
$lines += ""
$lines += "  [$(if($apiUp){'OK'}else{'FAIL'})] API /healthz: $(if($apiUp){'200 OK'}else{'unreachable'})"
$lines += ""

if ($failed.Count -gt 0) {
    $lines += "FAILED:"
    $failed | ForEach-Object { $lines += "  - $_" }
    Set-Content -Path $reportPath -Value ($lines -join "`n")
    Write-Warning "Health check failed. See $reportPath"
    exit 1
} else {
    $lines += "All checks passed."
    Set-Content -Path $reportPath -Value ($lines -join "`n")
    Write-Host "Health check passed. Report: $reportPath"
}
