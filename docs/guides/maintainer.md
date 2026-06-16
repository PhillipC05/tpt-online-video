# Maintainer Guide

This guide covers ongoing operational tasks for the person responsible for keeping a TPT Online Video instance healthy: upgrades, backups, monitoring, and routine housekeeping.

---

## Service overview

| Service | Binary / container | Depends on | Role |
|---------|-------------------|-----------|------|
| `tpt-api` | `services/api` | PostgreSQL, Redis, Storage | HTTP API, auth, metadata, WebSocket chat |
| `tpt-worker` | `services/worker` | PostgreSQL, Redis, FFmpeg, Storage | Transcoding pipeline |
| `tpt-live` | `services/live` | PostgreSQL, Redis, MediaMTX | Live stream lifecycle management |
| MediaMTX | External binary | — | RTMP ingest, HLS/WebRTC output |
| PostgreSQL | Database | — | Persistent metadata |
| Redis | Cache / queue | — | Transcoding job queue, pub/sub, rate limiting |
| Storage | Local FS / S3 / Wasabi | — | Video files, thumbnails, avatars |

---

## Health checks

**Quick check:**

```bash
# All three should return HTTP 200
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/api/v1/ping
```

**Docker Compose:**

```bash
docker compose -f docker-compose.prod.yml ps
# All containers should show "healthy"
```

**Systemd:**

```bash
systemctl status tpt-api tpt-worker tpt-live
```

**Admin UI:** Admin → System Status

---

## Backups

### PostgreSQL

Back up the database daily. Use `pg_dump` for logical backups:

```bash
pg_dump -U tpt -Fc tpt > tpt-$(date +%F).dump
```

Restore with:

```bash
pg_restore -U tpt -d tpt tpt-2024-01-01.dump
```

For Docker Compose deployments:

```bash
docker exec tpt-postgres pg_dump -U tpt -Fc tpt > tpt-$(date +%F).dump
```

**Retention recommendation:** keep 7 daily, 4 weekly, 12 monthly.

### Storage (media files)

If using local storage (`/opt/tpt/data/storage`), sync to an off-site location:

```bash
rsync -avz /opt/tpt/data/storage/ backup-server:/backups/tpt-media/
```

If using S3/Wasabi, enable bucket versioning and/or cross-region replication in the provider console.

### Redis

Redis is ephemeral by design — its queue drains during normal operation and its cache is rebuildable. For production you may still want a snapshot:

```bash
redis-cli BGSAVE
# Backup /var/lib/redis/dump.rdb
```

---

## Upgrades

### Docker Compose

```bash
git pull
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

`up -d` performs a rolling replacement: containers are restarted one at a time. The API will be briefly unreachable during the restart.

### Systemd (binary)

1. Download the new release archive.
2. Review the release notes for schema changes.
3. Stop services: `sudo systemctl stop tpt-api tpt-worker tpt-live`
4. Replace binaries in `/opt/tpt/bin/`.
5. Run migrations if needed: `sudo -u tpt /opt/tpt/bin/tpt-api migrate`
6. Start services: `sudo systemctl start tpt-api tpt-worker tpt-live`
7. Verify health endpoints.

---

## Database migrations

Migrations live in `migrations/` and are applied automatically at API startup. To run them manually:

```bash
# Systemd
sudo -u tpt /opt/tpt/bin/tpt-api migrate

# Docker
docker compose exec api /app/tpt-api migrate
```

Always take a database backup before running migrations on production.

---

## Log management

**Docker Compose** — logs go to Docker's logging driver (default: `json-file`). Rotate with Docker's built-in log rotation or ship to a logging aggregator:

```yaml
# docker-compose.prod.yml — add under each service
logging:
  driver: "json-file"
  options:
    max-size: "50m"
    max-file: "5"
```

**Systemd** — logs go to journald:

```bash
journalctl -u tpt-api --since "1 hour ago"
journalctl -u tpt-worker -n 100
```

Persist logs beyond the default journal size by setting `SystemMaxUse=500M` in `/etc/systemd/journald.conf`.

---

## Monitoring

### Prometheus metrics

The worker exposes Prometheus metrics on a dedicated port (default `:9090`):

```
GET http://localhost:9090/metrics
```

Scrape with Prometheus and use Grafana for dashboards. Key metrics:

| Metric | Description |
|--------|-------------|
| `tpt_transcode_jobs_total` | Total transcoding jobs processed |
| `tpt_transcode_duration_seconds` | Histogram of job durations |
| `tpt_transcode_errors_total` | Failed transcoding jobs |
| `tpt_live_streams_active` | Current live stream count |

### Alerts to configure

- API `readyz` returning non-200 for more than 60 seconds
- Redis memory usage above 80%
- PostgreSQL connection pool saturation
- Transcoding error rate above 5% over 15 minutes
- Disk space for storage volume below 20%

---

## Storage management

### Checking disk usage

```bash
du -sh /opt/tpt/data/storage/
```

### Cleaning up incomplete uploads

Incomplete upload sessions accumulate if users abandon uploads. Periodically clean up:

```sql
-- Identify old incomplete sessions (older than 7 days)
SELECT id, created_at FROM upload_sessions
WHERE status = 'uploading'
  AND created_at < NOW() - INTERVAL '7 days';

-- Mark them cancelled (soft cleanup)
UPDATE upload_sessions
SET status = 'cancelled'
WHERE status = 'uploading'
  AND created_at < NOW() - INTERVAL '7 days';
```

Then delete the orphaned raw files from storage (match by session ID prefix in the storage bucket).

### Cleaning up deleted videos

Soft-deleted videos remain in storage. To reclaim space, identify and remove their storage objects:

```sql
SELECT id, storage_path FROM videos WHERE deleted_at IS NOT NULL;
```

Delete the corresponding objects from storage, then hard-delete the rows if desired.

---

## Secret rotation

See [docs/security.md — Secret rotation procedures](../security.md#secret-rotation-procedures) for step-by-step instructions for rotating:

- JWT secret
- Live hook secret
- PostgreSQL password
- S3/MinIO credentials
- Redis password

---

## Audit log review

Review the audit log monthly (or after any incident):

- Look for unusual patterns: many actions by a single actor in a short period
- Verify that moderator actions are appropriate
- Confirm appeals are being handled promptly

**Admin → Audit Log** → filter by date range → export or note relevant entries.

---

## Dependency updates

Check for updates to Go dependencies and frontend packages quarterly:

```bash
# Go — list available updates
go list -u -m all

# Frontend
cd apps/web
pnpm outdated
```

Apply updates in a branch, run the test suite, and review the changelog for breaking changes before merging.

**External dependencies to monitor for security patches:**
- Go runtime
- PostgreSQL
- Redis
- FFmpeg
- MediaMTX

---

## Incident response

### Service is down

1. Check health endpoints — which service is unhealthy?
2. Check logs for the failing service.
3. Check that PostgreSQL and Redis are reachable.
4. Restart the affected service and monitor for recurrence.

### Transcoding queue is stuck

1. Admin → System Status — check active jobs.
2. Inspect worker logs: `journalctl -u tpt-worker -n 100` or `docker compose logs worker`.
3. Check FFmpeg is installed and accessible at the configured path.
4. Manually requeue stuck jobs via the admin SQL if needed.

### Live stream not starting

1. Verify OBS settings match the RTMP URL and stream key exactly.
2. Check MediaMTX is running and listening on port 1935.
3. Confirm `LIVE_HOOK_SECRET` matches between MediaMTX config and the API environment.
4. Check `tpt-live` logs for hook call errors.

### Database is full

1. Check disk space on the PostgreSQL volume.
2. Run `VACUUM FULL` if bloat is suspected.
3. Clear old soft-deleted rows (videos, comments, upload sessions).
4. Expand storage if needed.
