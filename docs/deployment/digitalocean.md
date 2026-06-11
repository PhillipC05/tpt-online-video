# Deploying TPT Online Video on DigitalOcean

This guide walks through deploying TPT Online Video on a DigitalOcean Droplet (VPS).

## Prerequisites

- A DigitalOcean account
- The `doctl` CLI tool (optional, for automation)

## 1. Create a Droplet

**Minimum specs:**
- **Plan:** Basic (4 GB / 2 CPUs) — good for a small community instance
- **OS:** Ubuntu 24.04 LTS
- **Add block storage** (optional, for media storage scaling)
- **Enable monitoring** (optional but recommended)

**Cheaper option for testing:** Basic (2 GB / 1 CPU) Premium Intel.

```bash
doctl compute droplet create tpt-video \
  --region sgp1 \
  --size s-2vcpu-4gb \
  --image ubuntu-24-04-x64 \
  --ssh-keys YOUR_SSH_FINGERPRINT \
  --enable-monitoring
```

Or create via the DigitalOcean web console → Droplets → Create.

## 2. SSH into the Droplet

```bash
ssh root@YOUR_DROPLET_IP
```

## 3. Install Dependencies

```bash
# Update system
apt update && apt upgrade -y

# Install Docker & Docker Compose
curl -fsSL https://get.docker.com | bash

# Install Go (for building binaries — skip if using pre-built binaries)
wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Node.js (for building frontend — skip if using pre-built)
curl -fsSL https://deb.nodesource.com/setup_20.x | bash
apt install -y nodejs

# Install pnpm
npm install -g pnpm
```

## 4. Clone the Repository

```bash
git clone https://github.com/YOUR_ORG/tpt-online-video.git /opt/tpt
cd /opt/tpt
```

## 5. Choose Deployment Model

### Option A: Fully Containerized (Recommended for DigitalOcean)

Everything runs in Docker. Simplest setup.

```bash
# Build and start all services
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# Check status
docker compose -f docker-compose.prod.yml ps
```

### Option B: Hybrid (systemd + Docker)

Go services run natively, infrastructure in Docker.

```bash
# Start infrastructure
docker compose up -d postgres redis minio mediamtx

# Build Go services
cd /opt/tpt
go build -o /usr/local/bin/tpt-api ./services/api/cmd/tpt-api
go build -o /usr/local/bin/tpt-worker ./services/worker/cmd/tpt-worker
go build -o /usr/local/bin/tpt-live ./services/live/cmd/tpt-live

# Build frontend
cd /opt/tpt
pnpm install
pnpm build

# Install systemd services
bash infra/installer/linux/install.sh

# Start services
systemctl start tpt-api tpt-worker tpt-live
```

## 6. Configure Nginx (Containerized method — already included in docker-compose.prod.yml)

If using the hybrid method, install and configure Nginx:

```bash
apt install -y nginx certbot python3-certbot-nginx
cp /opt/tpt/infra/nginx/tpt.conf /etc/nginx/sites-available/tpt
ln -s /etc/nginx/sites-available/tpt /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

## 7. Set Up SSL with Let's Encrypt

```bash
certbot --nginx -d your-domain.com
```

For the containerized method, add SSL termination via a separate certbot container or at the DigitalOcean load balancer level (recommended).

## 8. Configure Firewall

```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 1935/tcp   # RTMP for live streaming
ufw enable
```

## 9. Set Up Domain & DNS

1. Point an A record (e.g., `video.yourdomain.com`) to your Droplet IP
2. If using DigitalOcean Spaces for storage, configure in `.env`

## 10. Monitoring (Recommended)

Enable DigitalOcean Monitoring in the Droplet settings, or use:

```bash
docker compose -f docker-compose.prod.yml logs -f
```

## Troubleshooting

| Issue | Check |
|-------|-------|
| API not starting | `docker compose logs tpt-api` — verify Postgres is healthy |
| Uploads failing | Check MinIO console at `http://YOUR_IP:9001` — verify buckets exist |
| Live streaming not working | Verify MediaMTX is running and port 1935 is open |
| Frontend shows 502 | Check if nginx is running and can reach tpt-api:8080 |