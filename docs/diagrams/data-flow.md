# Data Flow Diagram

```mermaid
C4Dynamic
  Person(user, "User", "Viewer or broadcaster")
  System_Ext(browser, "Browser", "Web client")

  System_Boundary(tpt, "TPT Online Video") {
    Container(api, "API", "Go")
    Container(worker, "Worker", "Go")
    ContainerDb(pg, "PostgreSQL", "Database")
    ContainerDb(redis, "Redis", "Queue/Cache")
    Container(s3, "Object Storage", "Files")
    Container(mediamtx, "MediaMTX", "Media server")
  }

  Rel(user, browser, "Interacts with", "HTTPS")
  Rel(browser, api, "HTTP requests", "JSON API")
  Rel(api, pg, "SQL queries", "TCP")
  Rel(api, redis, "Cache lookups / pub/sub", "TCP")
  Rel(api, s3, "Upload / download", "S3 API")
  Rel(api, redis, "Enqueue transcode job", "Redis Streams")
  Rel(worker, redis, "Claim / ack jobs", "Redis Streams")
  Rel(worker, s3, "Read raw / write HLS", "S3 API")
  Rel(worker, pg, "Update job / video status", "SQL")
  Rel(browser, s3, "Direct HLS playback", "Signed URLs")
  Rel(browser, mediamtx, "WebRTC playback", "WHIP/WHEP")