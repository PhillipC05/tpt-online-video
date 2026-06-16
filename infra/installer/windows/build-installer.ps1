<#
.SYNOPSIS
    Assemble build artefacts and compile the Inno Setup installer.
.DESCRIPTION
    1. Builds Go binaries (tpt-api, tpt-worker, tpt-live) for Windows amd64.
    2. Builds the frontend with Vite.
    3. Downloads/copies third-party binaries into deps\.
    4. Runs ISCC.exe to produce dist\installer\tpt-online-video-<version>-setup.exe.
.PARAMETER Version
    Installer version string (default: git tag or "1.0.0").
.PARAMETER SkipBuild
    Skip Go and frontend builds; use whatever is already in dist\bin\ and dist\web\.
.PARAMETER IsccPath
    Path to ISCC.exe. Defaults to the standard Inno Setup 6 install location.
.EXAMPLE
    .\build-installer.ps1 -Version "1.2.0"
#>
[CmdletBinding()]
param(
    [string]$Version    = $env:TPT_VERSION,
    [switch]$SkipBuild,
    [string]$IsccPath   = "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
)

$ErrorActionPreference = "Stop"
$ScriptDir  = $PSScriptRoot
$RepoRoot   = Resolve-Path (Join-Path $ScriptDir "..\..\..") | Select-Object -ExpandProperty Path
$DistDir    = Join-Path $ScriptDir "dist"
$DepsDir    = Join-Path $ScriptDir "deps"

if (-not $Version) {
    $Version = git -C $RepoRoot describe --tags --abbrev=0 2>$null
    if (-not $Version) { $Version = "1.0.0" }
    $Version = $Version.TrimStart("v")
}

Write-Host "==> Building TPT Online Video installer v$Version"

# ── 1. Build Go binaries ─────────────────────────────────────────────────────
if (-not $SkipBuild) {
    Write-Host "==> Building Go binaries (GOOS=windows GOARCH=amd64)"
    $binDist = Join-Path $DistDir "bin"
    New-Item -ItemType Directory -Force -Path $binDist | Out-Null

    $services = @("api","worker","live")
    foreach ($svc in $services) {
        $pkg = "github.com/your-org/tpt-online-video/services/$svc/cmd/tpt-$svc"
        $out = Join-Path $binDist "tpt-$svc.exe"
        Write-Host "    tpt-$svc.exe"
        $env:GOOS   = "windows"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "0"
        go build -trimpath -ldflags "-s -w -X main.Version=$Version" -o $out $pkg
        if ($LASTEXITCODE -ne 0) { throw "go build failed for tpt-$svc" }
    }
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue

    # ── 2. Build frontend ─────────────────────────────────────────────────────
    Write-Host "==> Building frontend"
    $webDist = Join-Path $DistDir "web"
    Push-Location (Join-Path $RepoRoot "apps\web")
    pnpm install --frozen-lockfile
    pnpm build
    Pop-Location
    New-Item -ItemType Directory -Force -Path $webDist | Out-Null
    Copy-Item -Recurse -Force (Join-Path $RepoRoot "apps\web\dist\*") $webDist
}

# ── 3. Verify / fetch third-party deps ───────────────────────────────────────
Write-Host "==> Checking dependencies in $DepsDir"

function Assert-Dep([string]$path, [string]$hint) {
    if (-not (Test-Path $path)) {
        Write-Warning "Missing: $path`n  $hint"
        Write-Warning "Run: .\fetch-deps.ps1  (or place the file manually)"
    }
}

Assert-Dep "$DepsDir\winsw\winsw.exe"         "Download from https://github.com/winsw/winsw/releases (WinSW-x64.exe)"
Assert-Dep "$DepsDir\pgsql\bin\initdb.exe"    "Extract PostgreSQL Windows zip from https://www.enterprisedb.com/download-postgresql-binaries"
Assert-Dep "$DepsDir\redis\redis-server.exe"  "Download from https://github.com/tporadowski/redis/releases"
Assert-Dep "$DepsDir\minio\minio.exe"         "Download from https://dl.min.io/server/minio/release/windows-amd64/minio.exe"
Assert-Dep "$DepsDir\ffmpeg\ffmpeg.exe"       "Download gyan.dev essentials build and extract ffmpeg.exe + ffprobe.exe"
Assert-Dep "$DepsDir\ffmpeg\ffprobe.exe"      "Same gyan.dev archive"
Assert-Dep "$DepsDir\mediamtx\mediamtx.exe"  "Download from https://github.com/bluenviron/mediamtx/releases"
Assert-Dep "$DepsDir\mediamtx\mediamtx.yml"  "Same MediaMTX release archive"

# ── 4. Compile installer ─────────────────────────────────────────────────────
if (-not (Test-Path $IsccPath)) {
    throw "ISCC.exe not found at '$IsccPath'. Install Inno Setup 6 from https://jrsoftware.org/isinfo.php"
}

New-Item -ItemType Directory -Force -Path (Join-Path $DistDir "installer") | Out-Null

Write-Host "==> Compiling installer with Inno Setup"
$env:TPT_VERSION = $Version
& $IsccPath (Join-Path $ScriptDir "tpt-setup.iss")
if ($LASTEXITCODE -ne 0) { throw "ISCC compilation failed" }

$artifact = Join-Path $DistDir "installer\tpt-online-video-$Version-setup.exe"
Write-Host "==> Done: $artifact"
