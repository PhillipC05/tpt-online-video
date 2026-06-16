# Installer Guide

This document covers the three installation methods for TPT Online Video: Docker Compose (recommended), the self-contained Linux installer, and the Windows installer.

---

## Method 1 — Docker Compose (recommended)

The fully-containerised stack requires only Docker and Docker Compose. All dependencies (PostgreSQL, Redis, MinIO, MediaMTX) are bundled.

### Prerequisites

- Docker Engine 24+
- Docker Compose v2+

### Quick start

```bash
# Clone the repository
git clone https://github.com/your-org/tpt-online-video.git
cd tpt-online-video

# Copy and edit the environment file
cp .env.example .env
$EDITOR .env

# Build and start all services
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

The API is available at `http://localhost:8080` and the frontend at `http://localhost:80`.

### Key environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `JWT_SECRET` | Yes | — | Random 256-bit secret (`openssl rand -hex 32`) |
| `ADMIN_EMAIL` | Yes | — | Seed admin account email |
| `ADMIN_PASSWORD` | Yes | — | Seed admin account password |
| `POSTGRES_PASSWORD` | Yes | — | PostgreSQL password |
| `CORS_ALLOWED_ORIGINS` | Yes | — | Comma-separated list of allowed frontend origins |
| `APP_BASE_URL` | Yes | — | Public base URL (e.g. `https://video.example.com`) |
| `LIVE_HOOK_SECRET` | Yes | — | Secret shared between API and MediaMTX |
| `APP_ENV` | No | `development` | Set to `production` to enforce JWT secret validation |
| `STORAGE_PROVIDER` | No | `local` | `local`, `s3`, or `wasabi` |

See `.env.example` for the full list.

### Checking service health

```bash
# All containers should show "healthy"
docker compose -f docker-compose.prod.yml ps

# API health endpoints
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### Viewing logs

```bash
docker compose -f docker-compose.prod.yml logs -f api
docker compose -f docker-compose.prod.yml logs -f worker
docker compose -f docker-compose.prod.yml logs -f live
```

### Stopping and updating

```bash
# Stop
docker compose -f docker-compose.prod.yml down

# Pull latest and rebuild
git pull
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

---

## Method 2 — Linux self-contained installer

The Linux installer unpacks pre-compiled Go binaries and registers them as systemd services. PostgreSQL, Redis, FFmpeg, and MediaMTX must be installed separately.

### Prerequisites

| Dependency | Minimum version | Install |
|------------|----------------|---------|
| Linux (systemd) | Debian 12 / Ubuntu 22.04 / RHEL 9 | — |
| PostgreSQL | 15+ | `apt install postgresql` |
| Redis | 7+ | `apt install redis-server` |
| FFmpeg | 6+ | `apt install ffmpeg` |
| MediaMTX | 1.x | [GitHub releases](https://github.com/bluenviron/mediamtx/releases) |

### Installation steps

1. **Download the release archive** and extract it:

   ```bash
   tar -xzf tpt-online-video-linux-amd64.tar.gz
   cd tpt-online-video-linux-amd64
   ```

2. **Create the PostgreSQL database and user**:

   ```bash
   sudo -u postgres psql <<EOF
   CREATE USER tpt WITH PASSWORD 'changeme';
   CREATE DATABASE tpt OWNER tpt;
   EOF
   ```

3. **Run the installer** (requires root):

   ```bash
   sudo bash infra/installer/linux/install.sh
   ```

   This script:
   - Creates a `tpt` system user with no login shell
   - Creates `/opt/tpt/{bin,data,config}` directories
   - Installs `tpt-api`, `tpt-worker`, and `tpt-live` binaries
   - Installs and enables `tpt-api.service`, `tpt-worker.service`, and `tpt-live.service`
   - Starts all three services

4. **Edit the configuration file**:

   ```bash
   sudo $EDITOR /opt/tpt/config/config.yaml
   ```

   At minimum update:
   - `database.password` — match your PostgreSQL password
   - `live.hook_secret` — random value shared with MediaMTX
   - `server.host` / base URL

5. **Run database migrations**:

   ```bash
   sudo -u tpt /opt/tpt/bin/tpt-api migrate
   ```

6. **Restart services**:

   ```bash
   sudo systemctl restart tpt-api tpt-worker tpt-live
   ```

### Service management

```bash
# Status
sudo systemctl status tpt-api tpt-worker tpt-live

# Logs (follows)
sudo journalctl -u tpt-api -f
sudo journalctl -u tpt-worker -f
sudo journalctl -u tpt-live -f

# Restart a service
sudo systemctl restart tpt-api

# Stop all
sudo systemctl stop tpt-api tpt-worker tpt-live
```

### Upgrade

1. Download and extract the new release archive.
2. Stop the services: `sudo systemctl stop tpt-api tpt-worker tpt-live`
3. Replace the binaries in `/opt/tpt/bin/`.
4. Run migrations if the release notes mention schema changes.
5. Start the services: `sudo systemctl start tpt-api tpt-worker tpt-live`

### Uninstall

```bash
sudo systemctl stop tpt-api tpt-worker tpt-live
sudo systemctl disable tpt-api tpt-worker tpt-live
sudo rm /etc/systemd/system/tpt-{api,worker,live}.service
sudo systemctl daemon-reload
sudo rm -rf /opt/tpt
sudo userdel tpt
```

---

## Method 3 — Windows self-contained installer

The Windows installer is a single `.exe` built with [Inno Setup 6](https://jrsoftware.org/isinfo.php).
It bundles all required components — Go binaries, frontend static assets, PostgreSQL (portable), Redis,
MinIO, FFmpeg, and MediaMTX — and registers everything as Windows services automatically.

### System requirements

| Requirement | Minimum |
|-------------|---------|
| Windows | 10 / 11 / Server 2019+ (64-bit) |
| Disk space | ~2 GB (binaries + initial database) |
| RAM | 2 GB minimum, 4 GB recommended |
| PowerShell | 5.1+ (built into Windows 10/11) |
| Administrator rights | Required for service registration |

No additional software needs to be pre-installed; all dependencies are bundled.

### Installation steps

1. **Download** `tpt-online-video-<version>-setup.exe` from the releases page.

2. **Right-click → Run as administrator**. Accept the UAC prompt.

3. **Follow the wizard:**
   - Accept the licence agreement.
   - Choose the installation directory (default: `C:\Program Files\TPT Online Video`).
   - Enter the initial configuration on the *Configuration* page:
     - Administrator email and password (min 12 chars)
     - PostgreSQL password for the `tpt` database user
   - A 96-hex-character JWT secret is auto-generated; see the config file to customise it.

4. **Click Install.** The wizard will:
   - Extract all components.
   - Initialise the PostgreSQL data directory (`initdb`).
   - Register and start `tpt-postgresql` and `tpt-redis` as Windows services.
   - Register and start `tpt-minio`, `tpt-api`, `tpt-worker`, and `tpt-live`.
   - Run database migrations.
   - Add Windows Firewall rules for ports 8080, 1935, 8888, and 8889.
   - Run a post-install health check.

5. **Edit the configuration file** if you need to customise anything (SMTP, S3, TLS, etc.):

   ```powershell
   notepad "$env:ProgramFiles\TPT Online Video\config\config.yaml"
   ```

   Then restart the affected services:

   ```powershell
   Restart-Service tpt-api, tpt-worker, tpt-live
   ```

### Services installed

| Service name | Description |
|---|---|
| `tpt-postgresql` | Bundled PostgreSQL 15 (managed via `pg_ctl`) |
| `tpt-redis` | Bundled Redis 7 for Windows |
| `tpt-minio` | Bundled MinIO object storage (ports 9000 / 9001) |
| `tpt-api` | TPT HTTP API (port 8080) |
| `tpt-worker` | Transcoding and background jobs |
| `tpt-live` | Live-streaming hook handler |

All services start automatically with Windows and restart on failure.

### Service management

```powershell
# Status of all TPT services
Get-Service tpt-postgresql, tpt-redis, tpt-minio, tpt-api, tpt-worker, tpt-live

# Restart a single service
Restart-Service tpt-api

# View recent errors
Get-EventLog -LogName Application -Source tpt-api -Newest 50

# Log files (winsw rotated logs)
Get-ChildItem "$env:ProgramFiles\TPT Online Video\logs"
```

### Upgrade

Use the bundled upgrade script — no uninstall required:

```powershell
# Extract the new release archive, then:
& "$env:ProgramFiles\TPT Online Video\scripts\upgrade.ps1" `
    -NewBinDir ".\new-release\bin" `
    -NewWebDir ".\new-release\web"
```

The script stops services, replaces binaries, runs migrations, and restarts services.
Alternatively, run the new `tpt-online-video-<version>-setup.exe` over the existing
installation — Inno Setup detects the prior version and performs an in-place upgrade.

### Uninstall

Open **Add or Remove Programs**, find *TPT Online Video*, and click **Uninstall**.

The uninstaller stops and deregisters all services, removes firewall rules, and deletes
the installation directory.

> **Data preservation:** The PostgreSQL data directory (`data\pgsql`) and MinIO storage
> (`data\minio`) are removed with the installation directory. Back up these directories
> before uninstalling if you need to preserve data.

### Building the installer from source

Requirements: Go 1.22+, Node.js 20+ with pnpm, Inno Setup 6.3+.

```powershell
# 1. Place third-party binaries in infra\installer\windows\deps\
#    (see build-installer.ps1 for the expected layout and download links)

# 2. Build binaries + frontend + compile installer
cd infra\installer\windows
$env:TPT_VERSION = "1.2.0"
.\build-installer.ps1

# Output: dist\installer\tpt-online-video-1.2.0-setup.exe
```

The `build-installer.ps1` script checks for missing deps and prints download URLs
for any that are absent.

---

## Post-installation checklist

Regardless of installation method, verify the following before going live:

- [ ] All three services (`tpt-api`, `tpt-worker`, `tpt-live`) are running and healthy
- [ ] `GET /healthz` and `GET /readyz` return HTTP 200
- [ ] `JWT_SECRET` / `config.yaml` `auth.jwt_secret` is a random value (not the dev default)
- [ ] `LIVE_HOOK_SECRET` matches the value in MediaMTX configuration
- [ ] `ADMIN_PASSWORD` has been changed from the example placeholder
- [ ] CORS origins are set to your actual frontend URL (no wildcards)
- [ ] HTTPS is configured at the reverse proxy level
- [ ] Database is not publicly accessible
- [ ] `APP_ENV=production` is set
- [ ] First login with the seed admin account succeeds
- [ ] A test video upload completes and transcodes successfully
- [ ] A test live stream connects via OBS and viewers can watch

See [docs/security.md](security.md) for the full pre-production security checklist.
