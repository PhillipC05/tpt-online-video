<#
.SYNOPSIS
    In-place upgrade: stop services, replace binaries, run migrations, restart.
.DESCRIPTION
    Run from an elevated PowerShell session after extracting the new release archive
    alongside the existing installation.
.PARAMETER NewBinDir
    Directory containing the new tpt-api.exe, tpt-worker.exe, tpt-live.exe binaries.
.PARAMETER NewWebDir
    Directory containing the new frontend static assets (optional).
.PARAMETER InstallDir
    Root install directory. Defaults to %ProgramFiles%\TPT Online Video.
#>
[CmdletBinding(SupportsShouldProcess)]
param(
    [Parameter(Mandatory)][string]$NewBinDir,
    [string]$NewWebDir,
    [string]$InstallDir = "$env:ProgramFiles\TPT Online Video"
)

$ErrorActionPreference = "Stop"
$BinDir = Join-Path $InstallDir "bin"

# ── Validate ─────────────────────────────────────────────────────────────────
foreach ($bin in @("tpt-api.exe","tpt-worker.exe","tpt-live.exe")) {
    if (-not (Test-Path (Join-Path $NewBinDir $bin))) {
        throw "Missing binary in NewBinDir: $bin"
    }
}

# ── Stop TPT application services ─────────────────────────────────────────────
Write-Host "==> Stopping TPT services"
Stop-Service tpt-live, tpt-worker, tpt-api -Force -ErrorAction SilentlyContinue

# ── Replace binaries ──────────────────────────────────────────────────────────
Write-Host "==> Installing new binaries"
foreach ($bin in @("tpt-api.exe","tpt-worker.exe","tpt-live.exe")) {
    Copy-Item (Join-Path $NewBinDir $bin) (Join-Path $BinDir $bin) -Force
}

# ── Replace frontend assets (optional) ───────────────────────────────────────
if ($NewWebDir -and (Test-Path $NewWebDir)) {
    Write-Host "==> Installing new frontend assets"
    $webDest = Join-Path $InstallDir "web"
    Remove-Item -Recurse -Force $webDest -ErrorAction SilentlyContinue
    Copy-Item -Recurse $NewWebDir $webDest
}

# ── Run database migrations ───────────────────────────────────────────────────
Write-Host "==> Running database migrations"
& (Join-Path $BinDir "tpt-api.exe") migrate
if ($LASTEXITCODE -ne 0) { throw "Database migration failed (exit $LASTEXITCODE)" }

# ── Restart TPT application services ─────────────────────────────────────────
Write-Host "==> Starting TPT services"
Start-Service tpt-api
Start-Service tpt-worker
Start-Service tpt-live

# ── Quick health check ────────────────────────────────────────────────────────
Write-Host "==> Health check"
& "$InstallDir\scripts\healthcheck.ps1" -InstallDir $InstallDir -Retries 6
if ($LASTEXITCODE -ne 0) {
    Write-Warning "Health check failed after upgrade. Review logs at $InstallDir\logs\"
    exit 1
}

Write-Host "==> Upgrade complete."
