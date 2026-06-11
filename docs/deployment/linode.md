# Deploying TPT Online Video on Linode (Akamai Cloud)

This guide walks through deploying TPT Online Video on a Linode instance.

## Prerequisites

- A Linode account
- The `linode-cli` CLI tool (optional)

## 1. Create a Linode

**Minimum specs:**
- **Plan:** Linode 4GB — 2 vCPUs, 4GB RAM — good for a small community instance
- **OS:** Ubuntu 24.04 LTS
- **Region:** Choose the one closest to your audience
- **Add a block storage volume** for media files (optional but recommended)

**Cheaper option:** Linode 2GB for testing (2 vCPUs, 2GB RAM).

```bash
linode-cli linodes create \
  --label tpt-video \
  --region us-east \
  --type g6-standard-2 \
  --image linode/ubuntu24.04 \
  --root_pass YOUR_ROOT_PASSWORD
```

Or create via Linode Cloud Manager.

## 2. SSH into the Linode

```bash
ssh root@YOUR_LINODE_IP
```

## 3. Install Dependencies

```bash
# Update system
apt update && apt upgrade -y

# Install Docker & Docker Compose
curl -fsSL https://get.docker.com | bash

# Install Go
wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Node.js & pnpm
curl -fsSL https://deb.nodesource.com/setup_20.x | bash
apt install -y nodejs
npm install -g pnpm
```

## 4. Clone and Deploy

```bash
git clone https://github.com/YOUR_ORG/tpt-online-video.git /opt/tpt
cd /opt/tpt

# Fully containerized (recommended)
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

## 5. Linode-Specific Configuration

### Linode Firewall (Cloud Firewall)

Create a Cloud Firewall in the Linode Cloud Manager:

| Ingress Rule | Protocol | Ports | Sources |
|---|---|---|---|
| SSH | TCP | 22 | YOUR_IP |
| HTTP | TCP | 80 | 0.0.0.0/0 |
| HTTPS | TCP | 443 | 0.0.0.0/0 |
| RTMP | TCP | 1935 | 0.0.0.0/0 |

### Linode Object Storage (alternative to MinIO)

Linode offers S3-compatible Object Storage. To use it instead of the bundled MinIO:

1. Create a bucket in Linode Cloud Manager (e.g., `tpt-media`)
2. Generate an access key
3. Set environment variables in `.env`:

```env
TPT_STORAGE_PROVIDER=s3
TPT_STORAGE_S3_ENDPOINT=https://us-east-1.linodeobjects.com
TPT_STORAGE_S3_REGION=us-east-1
TPT_STORAGE_S3_BUCKET=tpt-media
TPT_STORAGE_S3_ACCESS_KEY=YOUR_ACCESS_KEY
TPT_STORAGE_S3_SECRET_KEY=YOUR_SECRET_KEY
```

### NodeBalancer (for high availability)

If you expect heavy traffic, set up a Linode NodeBalancer:

1. Create a NodeBalancer
2. Add your Linode as a backend node (port 80 or 443)
3. Configure SSL termination at the NodeBalancer
4. Point your domain to the NodeBalancer IP

## 6. Set Up SSL

```bash
# Install certbot
apt install -y certbot python3-certbot-nginx

# Get certificate (replace with your domain)
certbot --nginx -d video.yourdomain.com
```

## 7. Hardware Scaling Tips

| Scale Level | Linode Plan | Estimated Users |
|---|---|---|
| Testing | Linode 2GB | 1-10 |
| Small community | Linode 4GB | 10-100 |
| Medium | Linode 8GB | 100-500 |
| Large | Linode 16GB + Block Storage | 500+ |

## Troubleshooting

| Issue | Check |
|-------|-------|
| Container won't start | `docker compose -f docker-compose.prod.yml logs` |
| Out of disk space | `du -sh /var/lib/docker/volumes/` — Linode plans come with limited storage; attach Block Storage for media |
| Slow transcoding | Upgrade to a plan with dedicated CPUs (Linode Dedicated CPU plans) |