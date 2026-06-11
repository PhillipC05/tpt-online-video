# Component Diagram

```mermaid
C4Container
  Person(user, "User", "Viewer or broadcaster")

  System_Boundary(tpt, "TPT Online Video") {
    Container(web, "Web UI", "React, TypeScript, Vite", "Serves the frontend application")
    Container(api, "API", "Go", "HTTP API, auth, metadata, search, moderation")
    Container(worker, "Worker", "Go", "Consumes transcode jobs, runs FFmpeg")
    Container(live, "Live Helper", "Go", "Coordinates MediaMTX lifecycle and DVR")
    ContainerDb(pg, "PostgreSQL", "Relational DB", "Metadata, auth, search, comments, moderation")
    ContainerDb(redis, "Redis", "Key-value + Streams", "Queue, cache, pub/sub")
    Container(s3, "Object Storage", "MinIO / S3 / Wasabi", "Raw uploads, HLS segments, thumbnails")
    Container(mediamtx, "MediaMTX", "Media server", "RTMP ingest, HLS/WebRTC output")
  }

  Rel(user, web, "Uses", "HTTPS")
  Rel(web, api, "API calls", "HTTPS")
  Rel(api, pg, "Reads/writes", "SQL")
  Rel(api, redis, "Caches, publishes", "TCP")
  Rel(api, s3, "Stores/retrieves media", "HTTP/S3 API")
  Rel(api, worker, "Enqueues jobs", "Redis Streams")
  Rel(worker, s3, "Reads/writes media", "HTTP/S3 API")
  Rel(worker, pg, "Updates status", "SQL")
  Rel(api, mediamtx, "Manages streams", "API")
  Rel(live, mediamtx, "Coordinates lifecycle", "API")
  Rel(user, mediamtx, "RTMP ingest", "RTMP")
  Rel(user, mediamtx, "WebRTC watch", "WHIP/WHEP")