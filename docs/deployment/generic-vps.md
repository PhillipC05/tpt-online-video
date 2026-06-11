# Deploying TPT Online Video on a Generic VPS

This guide works for any Ubuntu 22.04/24.04 or Debian 12 VPS from providers like Hetzner, Vultr, OVHcloud, AWS EC2, Google Cloud, or any other standard VPS.

## Prerequisites

- A VPS running Ubuntu 22.04+ or Debian 12+
- SSH access (root or sudo user)
- A domain name pointed to your VPS IP (recommended but optional for testing)

## 1. SSH into Your VPS

```bash
ssh root@YOUR_VPS_IP
```

## 2. System Preparation

```bash
# Update system packages
apt update && apt upgrade -y

# Install essential tools
apt install -y curl git ufw

# Set hostname (optional)
hostnamectl set-hostname video.yourdomain.com
```

## 3. Install Docker & Docker Compose

```bash
curl -fsSL https://get.docker.com | bash
```

## 4. Install Go (for building from source)

```bash
GO_VERSION=$(curl -sL https://go.dev/VERSION?m=text | head -1)
wget "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz"
rm -rf /usr/local/go && tar -C /usr/local -xzf "${GO_VERSION}.linux-amd64.tar.gz"
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
source /etc/profile.d/go.sh
```

## 5. Clone and Build

```bash
# Clone the repository
git clone https://github.com/YOUR_ORG/tpt-online-video.git /opt/tpt
cd /opt/tpt

# Choose your deployment method

# Method 1: Fully containerized (easiest)
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# Method 2: Hybrid (systemd + Docker)
# See the Linux installer guide in infra/installer/linux/
```

## 6. Firewall Configuration

```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp        # SSH
ufw allow 80/tcp        # HTTP
ufw allow 443/tcp       # HTTPS
ufw allow 1935/tcp      # RTMP (live streaming)
ufw enable
```

## 7. SSL with Let's Encrypt

For the fully containerized deployment, the nginx container serves on port 80. Use certbot on the host to obtain certificates and mount them into the nginx container.

```bash
# Install certbot
apt install -y certbot

# Obtain certificate (standalone mode — stops nginx temporarily)
certbot certonly --standalone -d video.yourdomain.com

# Create a directory for certs accessible by Docker
mkdir -p /opt/tpt/certs
cp /etc/letsencrypt/live/video.yourdomain.com/fullchain.pem /opt/tpt/certs/
cp /etc/letsencrypt/live/video.yourdomain.com/privkey.pem /opt/tpt/certs/

# Add SSL volume mount to docker-compose.prod.yml
# Uncomment the nginx SSL port mapping and volume mounts
```

## 8. Storage Considerations

| Storage Type | Use Case | Configuration |
|---|---|---|
| Local disk (default) | Testing, single-user | No config needed |
| VPS block storage | Growing media library | Mount to `/mnt/media`, symlink to Docker volume |
| S3-compatible (MinIO) | Self-hosted object storage | Use bundled MinIO container |
| S3-compatible (external) | Production with backups | Set `TPT_STORAGE_PROVIDER=s3` with your provider's endpoint |

## 9. Backup Strategy

```bash
# Backup PostgreSQL
docker exec tpt-postgres pg_dump -U tpt tpt > /opt/backups/tpt-$(date +%Y%m%d).sql

# Backup Redis
docker exec tpt-redis redis-cli SAVE
cp /var/lib/docker/volumes/tpt-redis-data/_data/dump.rdb /opt/backups/

# Backup MinIO data
# Use mc (MinIO client) to sync to another bucket or local storage
```

## 10. Monitoring

```bash
# View logs
docker compose -f docker-compose.prod.yml logs -f --tail=100

# Check resource usage
docker stats

# Set up log rotation
echo '/var/lib/docker/containers/*/*.log {
  rotate 7
  daily
  compress
  missingok
  delaycompress
  copytruncate
}' > /etc/logrotate.d/docker-containers
```

## Provider-Specific Notes

| Provider | Notes |
|---|---|
| **Hetzner** | Enable the firewall in Hetzner Cloud Console. Use CX22 or higher for transcoding. |
| **Vultr** | High-Frequency instances work well for transcoding. Block storage available. |
| **OVHcloud** | Public Cloud instances with additional disks for storage. |
| **AWS EC2** | At minimum t3.medium. Use EBS gp3 volumes. S3 for storage instead of MinIO. |
| **Google Cloud** | e2-medium minimum. Use Cloud Storage instead of MinIO. |