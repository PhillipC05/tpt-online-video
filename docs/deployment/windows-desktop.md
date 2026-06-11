# Running TPT Online Video on Windows Desktop

This guide covers running TPT Online Video on a single Windows machine — ideal for personal use, testing, or development.

## Prerequisites

- **Windows 10/11** with WSL2 or Docker Desktop
- **Docker Desktop** for Windows (enables WSL2 backend)
- **Go 1.22+** (optional, if building from source)
- **Node.js 20+** (optional, if building frontend from source)

## 1. Install Docker Desktop

1. Download from [docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop/)
2. Install with WSL2 backend enabled
3. After installation, open PowerShell as Administrator and run:

```powershell
wsl --set-default-version 2
```

## 2. Clone the Repository

```powershell
# Using Git Bash or PowerShell
cd C:\Projects
git clone https://github.com/YOUR_ORG/tpt-online-video.git tpt-online-video
cd tpt-online-video
```

## 3. Choose Deployment Method

### Option A: Fully Containerized (Easiest — Everything in Docker)

```powershell
# Build and start all services
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# Verify all containers are running
docker compose -f docker-compose.prod.yml ps

# View logs
docker compose -f docker-compose.prod.yml logs -f
```

This runs everything — Postgres, Redis, MinIO, MediaMTX, API, Worker, Live, and Nginx — in Docker containers. Access the app at `http://localhost`.

### Option B: Hybrid (Docker for infra, native for services)

This is better for development — you can edit and restart services quickly.

```powershell
# Start infrastructure only
docker compose up -d postgres redis minio mediamtx

# In separate terminals, run each service:
cd services/api
go run .\cmd\tpt-api

cd services/worker
go run .\cmd\tpt-worker

cd services/live
go run .\cmd\tpt-live

# Frontend:
cd apps\web
npm install
npm run dev
```

## 4. Windows-Specific Configuration

### Storage Paths

By default, MinIO stores data in a Docker volume. If you want to use local storage instead of S3/MinIO:

1. Set `TPT_STORAGE_PROVIDER=local` in your `.env` file
2. API will store files in a local `./data/storage` directory (created automatically)

### Firewall

Windows Defender Firewall may block connections. Allow the following ports:

- `8080` (API)
- `5173` (Vite dev server)
- `1935` (RTMP for live streaming)
- `8888` (MediaMTX HLS)

```powershell
New-NetFirewallRule -DisplayName "TPT API" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow
New-NetFirewallRule -DisplayName "TPT RTMP" -Direction Inbound -Protocol TCP -LocalPort 1935 -Action Allow
```

### Performance Settings

For the Worker service (which runs FFmpeg), ensure Docker Desktop has adequate resources:

1. Open Docker Desktop → Settings → Resources
2. Set CPUs to at least 4
3. Set Memory to at least 4GB
4. Apply & Restart

## 5. Accessing the App

| Service | URL |
|---|---|
| Frontend | `http://localhost` (containerized) or `http://localhost:5173` (dev) |
| MinIO Console | `http://localhost:9001` (credentials: tpt / tpt123456) |
| API | `http://localhost:8080` |

## 6. Running as Windows Services (Optional)

If you want the Go services to start automatically with Windows, use winsw:

```powershell
# Download winsw
Invoke-WebRequest -Uri "https://github.com/winsw/winsw/releases/download/v3.0.0/WinSW-x64.exe" -OutFile ".\bin\winsw.exe"

# Run the installer
powershell -ExecutionPolicy Bypass -File .\infra\installer\windows\install.ps1
```

## 7. Stopping Everything

```powershell
# Containerized
docker compose -f docker-compose.prod.yml down

# Hybrid
docker compose down
```

## Troubleshooting

| Issue | Solution |
|---|---|
| Port already in use | Check `netstat -aon \| findstr :PORT` and stop the conflicting process |
| Docker not starting | Ensure WSL2 is installed: `wsl --set-default-version 2` |
| FFmpeg errors in worker | The API Docker image includes FFmpeg. For native mode, install FFmpeg via `winget install FFmpeg` |
| File permission errors | Docker Desktop on Windows uses Linux containers — ensure Docker volumes have correct permissions |
| High CPU usage | Reduce transcoding resolution or limit concurrent jobs in config |