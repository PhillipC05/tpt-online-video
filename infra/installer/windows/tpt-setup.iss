; TPT Online Video — Windows Self-Contained Installer
; Requires Inno Setup 6.3+
;
; Build with:
;   set TPT_VERSION=1.0.0
;   ISCC.exe infra\installer\windows\tpt-setup.iss
;
; Before building, run build-installer.ps1 to assemble dist\ and deps\.

#define MyAppName      "TPT Online Video"
#define MyAppVersion   GetEnv("TPT_VERSION")
#if MyAppVersion == ""
  #define MyAppVersion "1.0.0"
#endif
#define MyAppPublisher "TPT"
#define MyAppURL       "https://github.com/your-org/tpt-online-video"

[Setup]
AppId={{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={commonpf64}\TPT Online Video
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
OutputDir=dist\installer
OutputBaseFilename=tpt-online-video-{#MyAppVersion}-setup
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
MinVersion=10.0
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\bin\tpt-api.exe
DisableProgramGroupPage=yes
RestartIfNeededByRun=no
CloseApplications=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Dirs]
Name: "{app}\bin"
Name: "{app}\config"
Name: "{app}\web"
Name: "{app}\data\storage"
Name: "{app}\data\tmp"
Name: "{app}\data\pgsql"
Name: "{app}\data\redis"
Name: "{app}\data\minio"
Name: "{app}\logs\tpt-api"
Name: "{app}\logs\tpt-worker"
Name: "{app}\logs\tpt-live"
Name: "{app}\logs\minio"
Name: "{app}\scripts"
Name: "{app}\pgsql"

[Files]
; ── TPT Go binaries ──────────────────────────────────────────────────────────
Source: "dist\bin\tpt-api.exe";    DestDir: "{app}\bin"; Flags: ignoreversion
Source: "dist\bin\tpt-worker.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "dist\bin\tpt-live.exe";   DestDir: "{app}\bin"; Flags: ignoreversion

; ── Frontend static assets ───────────────────────────────────────────────────
Source: "dist\web\*"; DestDir: "{app}\web"; Flags: ignoreversion recursesubdirs createallsubdirs

; ── winsw service wrapper ────────────────────────────────────────────────────
Source: "deps\winsw\winsw.exe"; DestDir: "{app}\bin"; Flags: ignoreversion

; ── PostgreSQL portable ──────────────────────────────────────────────────────
Source: "deps\pgsql\*"; DestDir: "{app}\pgsql"; Flags: ignoreversion recursesubdirs createallsubdirs

; ── Redis for Windows (tporadowski/redis) ────────────────────────────────────
Source: "deps\redis\redis-server.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "deps\redis\redis-cli.exe";    DestDir: "{app}\bin"; Flags: ignoreversion
Source: "deps\redis\redis.conf";       DestDir: "{app}\config"; Flags: ignoreversion onlyifdoesntexist

; ── MinIO ────────────────────────────────────────────────────────────────────
Source: "deps\minio\minio.exe"; DestDir: "{app}\bin"; Flags: ignoreversion

; ── FFmpeg ───────────────────────────────────────────────────────────────────
Source: "deps\ffmpeg\ffmpeg.exe";  DestDir: "{app}\bin"; Flags: ignoreversion
Source: "deps\ffmpeg\ffprobe.exe"; DestDir: "{app}\bin"; Flags: ignoreversion

; ── MediaMTX ─────────────────────────────────────────────────────────────────
Source: "deps\mediamtx\mediamtx.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "deps\mediamtx\mediamtx.yml"; DestDir: "{app}\config"; Flags: ignoreversion onlyifdoesntexist

; ── winsw service definitions ────────────────────────────────────────────────
Source: "winsw\tpt-api.xml";    DestDir: "{app}\config"; Flags: ignoreversion
Source: "winsw\tpt-worker.xml"; DestDir: "{app}\config"; Flags: ignoreversion
Source: "winsw\tpt-live.xml";   DestDir: "{app}\config"; Flags: ignoreversion
Source: "winsw\tpt-minio.xml";  DestDir: "{app}\config"; Flags: ignoreversion

; ── Configuration template ───────────────────────────────────────────────────
Source: "config.windows.yaml"; DestDir: "{app}\config"; DestName: "config.yaml"; Flags: ignoreversion onlyifdoesntexist

; ── Helper scripts ───────────────────────────────────────────────────────────
Source: "setup-db.ps1";    DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "healthcheck.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "upgrade.ps1";     DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "uninstall.ps1";   DestDir: "{app}\scripts"; Flags: ignoreversion

[Run]
; 1. Initialise PostgreSQL cluster, create tpt DB/user, write config.yaml
Filename: "powershell.exe"; \
  Parameters: "-NonInteractive -ExecutionPolicy Bypass -File ""{app}\scripts\setup-db.ps1"" -InstallDir ""{app}"" -AdminEmail ""{code:GetAdminEmail}"" -AdminPassword ""{code:GetAdminPassword}"" -DBPassword ""{code:GetDBPassword}"""; \
  StatusMsg: "Initialising database..."; Flags: runhidden waituntilterminated

; 2. Replace {{INSTALL_DIR}} placeholder in winsw XMLs
Filename: "powershell.exe"; \
  Parameters: "-NonInteractive -ExecutionPolicy Bypass -Command ""Get-ChildItem '{app}\config\*.xml' | ForEach-Object { (Get-Content $_.FullName) -replace '\\{\\{INSTALL_DIR\\}\\}', '{app}' | Set-Content $_.FullName }"""; \
  StatusMsg: "Configuring service definitions..."; Flags: runhidden waituntilterminated

; 3. Register infrastructure services
Filename: "{app}\bin\winsw.exe"; Parameters: "install ""{app}\config\tpt-minio.xml""";  StatusMsg: "Registering MinIO service...";  Flags: runhidden waituntilterminated

; 4. Start infrastructure services (PostgreSQL was started by setup-db.ps1; Redis by redis service install)
Filename: "{app}\bin\winsw.exe"; Parameters: "start ""{app}\config\tpt-minio.xml"""; StatusMsg: "Starting MinIO..."; Flags: runhidden waituntilterminated

; 5. Run database migrations
Filename: "{app}\bin\tpt-api.exe"; Parameters: "migrate"; \
  StatusMsg: "Running database migrations..."; Flags: runhidden waituntilterminated

; 6. Register TPT application services
Filename: "{app}\bin\winsw.exe"; Parameters: "install ""{app}\config\tpt-api.xml""";    StatusMsg: "Registering API service...";    Flags: runhidden waituntilterminated
Filename: "{app}\bin\winsw.exe"; Parameters: "install ""{app}\config\tpt-worker.xml"""; StatusMsg: "Registering Worker service..."; Flags: runhidden waituntilterminated
Filename: "{app}\bin\winsw.exe"; Parameters: "install ""{app}\config\tpt-live.xml""";   StatusMsg: "Registering Live service...";   Flags: runhidden waituntilterminated

; 7. Start TPT application services
Filename: "{app}\bin\winsw.exe"; Parameters: "start ""{app}\config\tpt-api.xml""";    StatusMsg: "Starting API...";    Flags: runhidden waituntilterminated
Filename: "{app}\bin\winsw.exe"; Parameters: "start ""{app}\config\tpt-worker.xml"""; StatusMsg: "Starting Worker..."; Flags: runhidden waituntilterminated
Filename: "{app}\bin\winsw.exe"; Parameters: "start ""{app}\config\tpt-live.xml""";   StatusMsg: "Starting Live...";   Flags: runhidden waituntilterminated

; 8. Configure firewall rules
Filename: "netsh.exe"; Parameters: "advfirewall firewall add rule name=""TPT API"" protocol=TCP dir=in localport=8080 action=allow"; \
  StatusMsg: "Configuring firewall..."; Flags: runhidden waituntilterminated
Filename: "netsh.exe"; Parameters: "advfirewall firewall add rule name=""TPT RTMP"" protocol=TCP dir=in localport=1935 action=allow"; \
  StatusMsg: "Configuring firewall (RTMP)..."; Flags: runhidden waituntilterminated
Filename: "netsh.exe"; Parameters: "advfirewall firewall add rule name=""TPT HLS"" protocol=TCP dir=in localport=8888 action=allow"; \
  StatusMsg: "Configuring firewall (HLS)..."; Flags: runhidden waituntilterminated
Filename: "netsh.exe"; Parameters: "advfirewall firewall add rule name=""TPT WebRTC"" protocol=TCP dir=in localport=8889 action=allow"; \
  StatusMsg: "Configuring firewall (WebRTC)..."; Flags: runhidden waituntilterminated

; 9. Post-install health check
Filename: "powershell.exe"; \
  Parameters: "-NonInteractive -ExecutionPolicy Bypass -File ""{app}\scripts\healthcheck.ps1"" -InstallDir ""{app}"""; \
  StatusMsg: "Verifying installation..."; Flags: runhidden waituntilterminated

[UninstallRun]
Filename: "powershell.exe"; \
  Parameters: "-NonInteractive -ExecutionPolicy Bypass -File ""{app}\scripts\uninstall.ps1"" -InstallDir ""{app}"""; \
  Flags: runhidden waituntilterminated; RunOnceId: "StopAndUnregisterServices"

[UninstallDelete]
; Remove winsw log directories not cleaned by the uninstall script
Type: filesandordirs; Name: "{app}\logs"

[Code]
var
  ConfigPage: TInputQueryWizardPage;

function GetAdminEmail(Param: string): string;
begin
  Result := ConfigPage.Values[0];
end;

function GetAdminPassword(Param: string): string;
begin
  Result := ConfigPage.Values[1];
end;

function GetDBPassword(Param: string): string;
begin
  Result := ConfigPage.Values[2];
end;

procedure InitializeWizard;
begin
  ConfigPage := CreateInputQueryPage(
    wpSelectDir,
    'Initial Configuration',
    'Configure the administrator account and database.',
    'These values are written to config.yaml and can be changed later by editing ' +
    ExpandConstant('{commonpf64}') + '\TPT Online Video\config\config.yaml.');

  ConfigPage.Add('Administrator email address:', False);
  ConfigPage.Add('Administrator password (minimum 12 characters):', True);
  ConfigPage.Add('PostgreSQL password for the tpt database user:', True);

  ConfigPage.Values[0] := 'admin@example.com';
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID <> ConfigPage.ID then
    Exit;

  if Trim(ConfigPage.Values[0]) = '' then
  begin
    MsgBox('Administrator email is required.', mbError, MB_OK);
    Result := False;
    Exit;
  end;

  if Length(ConfigPage.Values[1]) < 12 then
  begin
    MsgBox('Administrator password must be at least 12 characters.', mbError, MB_OK);
    Result := False;
    Exit;
  end;

  if Trim(ConfigPage.Values[2]) = '' then
  begin
    MsgBox('PostgreSQL password is required.', mbError, MB_OK);
    Result := False;
    Exit;
  end;
end;
