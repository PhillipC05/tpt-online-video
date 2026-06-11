# Deployment

TPT Online Video supports two deployment models:

- **Fully containerized (recommended for production):** Everything runs in Docker containers via a single `docker-compose.prod.yml`
- **Hybrid (development/advanced):** Go services run natively via systemd, infrastructure runs in Docker

## Quick Start

### Model A: Fully Containerized (Recommended)

```bash
# Build images and start all services
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# Check status
docker compose -f docker-compose.prod.yml ps

# View logs
docker compose -f docker-compose.prod.yml logs -f tpt-api
```

Access the app at `http://localhost`.

### Model B: Hybrid (systemd + Docker)

```bash
# Start infrastructure
docker compose up -d postgres redis minio mediamtx

# Build Go binaries
cd services/api && go build -o /usr/local/bin/tpt-api ./cmd/tpt-api
cd services/worker && go build -o /usr/local/bin/tpt-worker ./cmd/tpt-worker
cd services/live && go build -o /usr/local/bin/tpt-live ./cmd/tpt-live

# Install and start systemd services
bash infra/installer/linux/install.sh
```

## Platform-Specific Guides

| Platform | Guide | Difficulty |
|---|---|---|
| **DigitalOcean** | [DigitalOcean Droplet](./deployment/digitalocean.md) | Easy |
| **Linode** | [Linode Instance](./deployment/linode.md) | Easy |
| **Generic VPS** | [Any Ubuntu/Debian VPS](./deployment/generic-vps.md) | Easy |
| **Windows Desktop** | [Windows 10/11](./deployment/windows-desktop.md) | Moderate |

## Infrastructure Services

| Service | Image | Purpose |
|---|---|---|
| PostgreSQL 16 | `postgres:16-alpine` | Metadata, auth, search, comments, moderation |
| Redis 7 | `redis:7-alpine` | Job queue, cache, pub/sub for chat |
| MinIO | `minio/minio:latest` | S3-compatible object storage |
| MediaMTX | `bluenviron/mediamtx:latest` | RTMP ingest, HLS/WebRTC output |
| Nginx | `nginx:alpine` | Reverse proxy, static file serving |

## Default Credentials

```text
Postgres: tpt / tpt
MinIO: tpt / tpt123456
```

Change these in `.env` before deploying to production.

## Configuration

All configuration is via environment variables (see `.env.example`):

| Variable | Default | Description |
|---|---|---|
| `POSTGRES_DB` | `tpt` | PostgreSQL database name |
| `POSTGRES_USER` | `tpt` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `tpt` | PostgreSQL password |
| `MINIO_ROOT_USER` | `tpt` | MinIO access key |
| `MINIO_ROOT_PASSWORD` | `tpt123456` | MinIO secret key |
| `TPT_STORAGE_PROVIDER` | `minio` | Storage backend (`minio`, `s3`, `local`) |
| `TPT_STORAGE_S3_ENDPOINT` | `http://minio:9000` | S3 endpoint URL |

## SSL / HTTPS

### Containerized deployment
Add SSL certificate paths as volumes to the nginx container in `docker-compose.prod.yml`, or terminate SSL at a load balancer (DigitalOcean LB, Linode NodeBalancer, Cloudflare).

### Hybrid deployment
```bash
apt install -y certbot python3-certbot-nginx
certbot --nginx -d video.yourdomain.com
```

## Storage backends

| Backend | Provider | Use Case |
|---|---|---|
| `minio` (default) | Bundled MinIO container | Self-hosted, no external dependencies |
| `s3` | Any S3-compatible provider (Wasabi, Backblaze, DigitalOcean Spaces, Linode Object Storage) | Production with backups, off-site storage |
| `local` | Local filesystem | Single-user dev/test on Windows |

## Volume mounts

| Volume | Container | Purpose | Backup priority |
|---|---|---|---|
| `tpt-postgres-data` | postgres | Database files | Critical |
| `tpt-redis-data` | redis | Redis persistence | Important |
| `tpt-minio-data` | minio | Media files | Critical |
| `tpt-worker-data` | worker | FFmpeg temp files | None |

## Production checklist

- [ ] Change all default credentials in `.env`
- [ ] Configure SSL/TLS certificate
- [ ] Set up firewall (allow only ports 22, 80, 443, 1935)
- [ ] Configure automated database backups
- [ ] Set up log rotation
- [ ] Configure monitoring (Uptime Kuma, Grafana, or provider monitoring)
- [ ] Choose a storage backend (MinIO vs external S3)
- [ ] Set up a swapfile if RAM < 4GB

## Docker image overview

| Image | Path | Base | Size |
|---|---|---|---|
| `tpt-api` | `services/api/Dockerfile` | `golang:1.22-alpine` → `alpine:3.19` | ~80MB |
| `tpt-worker` | `services/worker/Dockerfile` | `golang:1.22-alpine` → `alpine:3.19` | ~80MB |
| `tpt-live` | `services/live/Dockerfile` | `golang:1.22-alpine` → `alpine:3.19` | ~30MB |
| `tpt-web` | `apps/web/Dockerfile` | `node:20-alpine` → `nginx:alpine` | ~50MB |