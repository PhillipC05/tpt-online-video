<#
.SYNOPSIS
    Called by the Inno Setup installer to initialise PostgreSQL, Redis,
    and write the final config.yaml.
.PARAMETER InstallDir
    Root install directory, e.g. C:\Program Files\TPT Online Video
.PARAMETER AdminEmail
    Seed administrator email address.
.PARAMETER AdminPassword
    Seed administrator password.
.PARAMETER DBPassword
    Password for the PostgreSQL tpt user.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$InstallDir,
    [Parameter(Mandatory)][string]$AdminEmail,
    [Parameter(Mandatory)][string]$AdminPassword,
    [Parameter(Mandatory)][string]$DBPassword
)

$ErrorActionPreference = "Stop"
$PgsqlBin  = Join-Path $InstallDir "pgsql\bin"
$DataDir   = Join-Path $InstallDir "data\pgsql"
$ConfigDir = Join-Path $InstallDir "config"
$LogsDir   = Join-Path $InstallDir "logs"

# ── Generate JWT / hook secret ───────────────────────────────────────────────
$rng    = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$bytes  = New-Object byte[] 48
$rng.GetBytes($bytes)
$JwtSecret = [BitConverter]::ToString($bytes) -replace "-",""

# ── Initialise PostgreSQL cluster ────────────────────────────────────────────
if (-not (Test-Path (Join-Path $DataDir "PG_VERSION"))) {
    Write-Host "  initdb: creating cluster at $DataDir"
    & (Join-Path $PgsqlBin "initdb.exe") `
        --pgdata="$DataDir" `
        --encoding=UTF8 `
        --auth=md5 `
        --username=postgres `
        --pwfile=(New-TemporaryFile).FullName  # postgres superuser uses trust locally
    if ($LASTEXITCODE -ne 0) { throw "initdb failed (exit $LASTEXITCODE)" }
}

# ── Register PostgreSQL as a Windows service ─────────────────────────────────
$svcExists = Get-Service "tpt-postgresql" -ErrorAction SilentlyContinue
if (-not $svcExists) {
    Write-Host "  pg_ctl: registering tpt-postgresql service"
    & (Join-Path $PgsqlBin "pg_ctl.exe") register `
        -N "tpt-postgresql" `
        -D "$DataDir" `
        -S auto
    if ($LASTEXITCODE -ne 0) { throw "pg_ctl register failed (exit $LASTEXITCODE)" }
}

# ── Start PostgreSQL ─────────────────────────────────────────────────────────
Start-Service "tpt-postgresql"
# Wait up to 30 s for PostgreSQL to accept connections
$pgReady = $false
for ($i = 0; $i -lt 30; $i++) {
    $r = & (Join-Path $PgsqlBin "pg_isready.exe") -h localhost -p 5432 2>$null
    if ($LASTEXITCODE -eq 0) { $pgReady = $true; break }
    Start-Sleep -Seconds 1
}
if (-not $pgReady) { throw "PostgreSQL did not become ready within 30 seconds" }

# ── Create tpt database and user ─────────────────────────────────────────────
$psql = Join-Path $PgsqlBin "psql.exe"
$env:PGPASSWORD = ""   # superuser uses trust (local socket or localhost + md5 with password file)

$createUser = "DO `$`$ BEGIN " +
    "IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname='tpt') THEN " +
    "CREATE ROLE tpt LOGIN PASSWORD '$DBPassword'; " +
    "END IF; END `$`$;"
& $psql -U postgres -h localhost -c $createUser
if ($LASTEXITCODE -ne 0) { throw "Failed to create tpt role" }

$createDB = "SELECT 'CREATE DATABASE tpt OWNER tpt' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname='tpt')\gexec"
& $psql -U postgres -h localhost -c $createDB
if ($LASTEXITCODE -ne 0) { throw "Failed to create tpt database" }

# ── Register Redis as a Windows service ──────────────────────────────────────
$redisBin  = Join-Path $InstallDir "bin\redis-server.exe"
$redisConf = Join-Path $ConfigDir "redis.conf"
$redisSvc  = Get-Service "tpt-redis" -ErrorAction SilentlyContinue
if (-not $redisSvc) {
    Write-Host "  redis-server: registering tpt-redis service"
    & $redisBin --service-install --service-name tpt-redis `
        --port 6379 `
        --dir (Join-Path $InstallDir "data\redis") `
        --logfile (Join-Path $LogsDir "redis\redis.log") `
        --save 900 1 --save 300 10 --save 60 10000
    if ($LASTEXITCODE -ne 0) { throw "redis-server --service-install failed" }
}
Start-Service "tpt-redis"

# ── Write final config.yaml ───────────────────────────────────────────────────
$configTemplate = Join-Path $ConfigDir "config.yaml"
if (Test-Path $configTemplate) {
    $content = Get-Content $configTemplate -Raw
    $content = $content -replace '\{\{INSTALL_DIR\}\}',     ($InstallDir -replace '\\','\\')
    $content = $content -replace '\{\{ADMIN_EMAIL\}\}',     $AdminEmail
    $content = $content -replace '\{\{ADMIN_PASSWORD\}\}',  $AdminPassword
    $content = $content -replace '\{\{DB_PASSWORD\}\}',     $DBPassword
    $content = $content -replace '\{\{JWT_SECRET\}\}',      $JwtSecret
    Set-Content -Path $configTemplate -Value $content -Encoding UTF8
    Write-Host "  config.yaml written"
}

Write-Host "  setup-db complete"
