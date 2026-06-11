# Architecture

TPT Online Video is designed as a modular monolith backend with separate deployable processes for API, transcoding workers, and live helper functionality.

## Core components

```text
React Web UI
    |
    v
Go API
    |
    |--> PostgreSQL metadata/auth/search/moderation
    |--> Redis queue/cache/pubsub
    |--> Storage abstraction
    |       |--> Local filesystem
    |       |--> S3-compatible
    |       |--> Wasabi
    |
    |--> Search abstraction
    |       |--> PostgreSQL FTS first
    |       |--> Meilisearch later
    |
    |--> Transcoding queue
            |
            v
        Go Worker
            |
            v
          FFmpeg
            |
            v
        HLS renditions

OBS / Encoder
    |
    v
MediaMTX
    |
    |--> HLS live output
    |--> WebRTC/WHEP output
    |--> DVR sliding window
```

## Service boundaries

### API

The API owns:

- HTTP routing
- Auth/session handling
- User/profile metadata
- Video metadata
- Upload session orchestration
- Transcoding job creation
- Search API
- Comments API
- Moderation API
- Live stream metadata API
- WebSocket chat coordination

### Worker

The worker owns:

- Queue consumption
- FFmpeg process execution
- HLS rendition generation
- Thumbnail generation
- Media metadata extraction
- Transcoding progress reporting
- Retry/dead-letter handling

### Live helper

The live helper owns:

- MediaMTX lifecycle coordination
- Stream key generation and hashing
- Live stream start/end detection hooks
- DVR sliding-window coordination
- Live chat integration hooks

## Data flow: VOD

1. Browser creates upload session.
2. Browser uploads chunks.
3. API stores raw file through storage abstraction.
4. API creates video metadata and transcoding job.
5. Worker consumes job.
6. FFmpeg generates 1080p/720p/480p/360p HLS.
7. Worker uploads HLS manifests/segments.
8. Worker updates database progress/status.
9. Browser watches via Shaka Player using the master HLS manifest.

## Data flow: live

1. Broadcaster creates live stream.
2. API returns RTMP URL with stream key.
3. OBS publishes to MediaMTX.
4. MediaMTX outputs HLS and WebRTC.
5. Viewers watch through HLS or WebRTC.
6. Chat messages flow through WebSocket + Redis pub/sub.
7. DVR retains a sliding window of recent live segments.