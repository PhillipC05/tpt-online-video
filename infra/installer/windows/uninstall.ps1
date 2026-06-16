<#
.SYNOPSIS
    Called by the Inno Setup uninstaller to stop and deregister all services.
.PARAMETER InstallDir
    Root install directory.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$InstallDir
)

$ErrorActionPreference = "Continue"
$BinDir = Join-Path $InstallDir "bin"
$winsw  = Join-Path $BinDir "winsw.exe"
$PgsqlBin = Join-Path $InstallDir "pgsql\bin"

# ── Stop and unregister winsw-managed services ────────────────────────────────
$winswServices = @("tpt-live","tpt-worker","tpt-api","tpt-minio")
foreach ($svc in $winswServices) {
    $xmlPath = Join-Path $InstallDir "config\$svc.xml"
    Stop-Service $svc -Force -ErrorAction SilentlyContinue
    if (Test-Path $winsw) {
        & $winsw uninstall $xmlPath 2>$null
    }
}

# ── Stop and unregister Redis ─────────────────────────────────────────────────
Stop-Service "tpt-redis" -Force -ErrorAction SilentlyContinue
$redisBin = Join-Path $BinDir "redis-server.exe"
if (Test-Path $redisBin) {
    & $redisBin --service-uninstall --service-name tpt-redis 2>$null
}

# ── Stop and unregister PostgreSQL ────────────────────────────────────────────
Stop-Service "tpt-postgresql" -Force -ErrorAction SilentlyContinue
if (Test-Path (Join-Path $PgsqlBin "pg_ctl.exe")) {
    & (Join-Path $PgsqlBin "pg_ctl.exe") unregister -N "tpt-postgresql" 2>$null
}

# ── Remove firewall rules ─────────────────────────────────────────────────────
foreach ($rule in @("TPT API","TPT RTMP","TPT HLS","TPT WebRTC")) {
    netsh advfirewall firewall delete rule name="$rule" 2>$null | Out-Null
}

Write-Host "Services stopped and deregistered."
