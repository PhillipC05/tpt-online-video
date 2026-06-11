<#
.SYNOPSIS
    TPT Online Video — Windows Installer
.DESCRIPTION
    Installs the API, Worker, and Live services as Windows services
    using winsw (Windows Service Wrapper).
#>

$ErrorActionPreference = "Stop"
$InstallDir = "$env:ProgramFiles\TPT Online Video"
$BinDir = "$InstallDir\bin"
$DataDir = "$InstallDir\data"
$ConfigDir = "$InstallDir\config"

Write-Host "==> Creating directories"
New-Item -ItemType Directory -Force -Path $BinDir    | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir   | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

Write-Host "==> Copying binaries"
Copy-Item ".\bin\tpt-api.exe"    $BinDir -Force
Copy-Item ".\bin\tpt-worker.exe" $BinDir -Force
Copy-Item ".\bin\tpt-live.exe"   $BinDir -Force

Write-Host "==> Installing services"
$services = @("tpt-api", "tpt-worker", "tpt-live")
foreach ($svc in $services) {
    $xmlPath = "$InstallDir\$svc.xml"
    Copy-Item ".\infra\installer\windows\tpt-online-video.xml" $xmlPath -Force
    (Get-Content $xmlPath) -replace "{{SERVICE_NAME}}", $svc `
        -replace "{{EXECUTABLE}}", "$BinDir\$svc.exe" `
        -replace "{{DISPLAY_NAME}}", "TPT Online Video — $svc" | Set-Content $xmlPath

    & "$BinDir\winsw" install $xmlPath
    & "$BinDir\winsw" start $xmlPath
}

Write-Host "==> Done."