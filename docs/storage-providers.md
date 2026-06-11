# Storage Providers

TPT Online Video uses an abstracted storage interface so media can be stored locally, in MinIO/S3, or in Wasabi without changing application logic.

## Supported provider targets

| Provider | Use case |
|---|---|
| `local` | Development and single-machine installs |
| `s3` / `minio` | Docker Compose and S3-compatible deployments |
| `wasabi` | Production-compatible object storage |

## Local storage

Local storage writes objects under `LOCAL_STORAGE_ROOT`.

Example:

```bash
STORAGE_PROVIDER=local
LOCAL_STORAGE_ROOT=./data/storage
```

Recommended layout:

```text
data/storage/
  tpt-media/
    raw/{video_id}/upload.mp4
    hls/{video_id}/1080p/index.m3u8
    hls/{video_id}/1080p/seg-00001.ts
    thumbs/{video_id}/poster.jpg
  tpt-live/
    {stream_id}/hls/index.m3u8
  tpt-cache/
```

## S3-compatible storage

Example MinIO:

```bash
STORAGE_PROVIDER=s3
S3_ENDPOINT=http://localhost:9000
S3_BUCKET=tpt-media
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=tpt
S3_SECRET_ACCESS_KEY=tpt123456
S3_USE_PATH_STYLE=true
```

Example Wasabi-style deployment:

```bash
STORAGE_PROVIDER=wasabi
S3_ENDPOINT=https://s3.wasabisys.com
S3_BUCKET=your-bucket
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=your-access-key
S3_SECRET_ACCESS_KEY=your-secret-key
S3_USE_PATH_STYLE=false
```

## Object keys

The storage layer intentionally accepts buckets and object keys. Higher-level services are responsible for generating safe keys.

Recommended key patterns:

```text
raw/{videoId}/upload.mp4
hls/{videoId}/{rendition}/index.m3u8
hls/{videoId}/{rendition}/seg-%05d.ts
thumbs/{videoId}/poster.jpg
live/{streamId}/hls/index.m3u8
live/{streamId}/dvr/seg-%05d.ts
```

## Security notes

- Never trust client-provided object keys.
- Reject absolute paths and parent traversal.
- Use short-lived presigned URLs for uploads/downloads where appropriate.
- Rotate object storage credentials independently from application secrets.
- Do not store raw media in git or public repositories.