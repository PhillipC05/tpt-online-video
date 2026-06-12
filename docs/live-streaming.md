# Live Streaming

TPT Online Video supports live streaming via **MediaMTX** for RTMP ingest, HLS playback, and WebRTC low-latency playback.

## Architecture

```
OBS (RTMP push)  ──►  MediaMTX (:1935)  ──►  HLS (:8888)  ──►  Browser (Shaka Player)
                                        └─►  WebRTC (:8889) ──►  Browser (WHIP/WHEP)
                                               │
                                               ▼
                                        API Hooks ──►  Live Service ──►  PostgreSQL
```

1. User creates a live stream via the API, which generates a **stream key** (64-char hex, SHA-256 hashed for storage).
2. User configures OBS with the RTMP URL and stream key.
3. When OBS pushes to MediaMTX, MediaMTX calls the API's `on-publish` hook.
4. The API validates the stream key (by hash lookup) and marks the stream as `live`.
5. Viewers watch via HLS (`/live/{streamKey}/index.m3u8`) or WebRTC.
6. When OBS disconnects, MediaMTX calls `on-unpublish`, and the API marks the stream as `ended`.

## Media server

The project uses **MediaMTX** (formerly rtsp-simple-server) because it supports RTMP, HLS, WebRTC, and WHEP in a single cross-platform service.

Configuration is at `infra/docker/mediamtx/mediamtx.yml`.

## API endpoints

All live endpoints are under `/api/v1/live/`.

### Create a live stream

```
POST /api/v1/live/streams
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "My Live Stream",
  "description": "Optional description",
  "dvr_enabled": true,
  "dvr_window_seconds": 900
}
```

Response (201):
```json
{
  "success": true,
  "data": {
    "stream": { "id": "...", "title": "...", "status": "idle", ... },
    "stream_key": "abc123...64chars",
    "stream_key_url": "rtmp://localhost:1935/live/abc123..."
  }
}
```

> **Important**: The `stream_key` is shown only once at creation. Store it securely before navigating away.

### List your streams

```
GET /api/v1/live/streams
Authorization: Bearer <token>
```

### Get stream details

```
GET /api/v1/live/streams/{streamID}
Authorization: Bearer <token> (optional — owner sees all, public sees only live/ended)
```

### Update stream

```
PATCH /api/v1/live/streams/{streamID}
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "Updated Title",
  "description": "Updated description"
}
```

### Delete stream

```
DELETE /api/v1/live/streams/{streamID}
Authorization: Bearer <token>
```

### List currently live streams (public)

```
GET /api/v1/live/streams/live
```

### Get stream playback URLs

```
GET /api/v1/live/streams/{streamID}/urls
```

### MediaMTX hooks (internal, validated by X-Hook-Secret)

```
POST /api/v1/live/hooks/auth
POST /api/v1/live/hooks/on-publish
POST /api/v1/live/hooks/on-unpublish
```

## RTMP ingest

Example RTMP URL for OBS:

```text
rtmp://your-server:1935/live/{stream_key}
```

The API stores only a **SHA-256 hash** of the stream key. The plaintext key is returned once at stream creation.

## HLS live playback

Example HLS URL:

```text
http://your-server:8888/live/{stream_key}/index.m3u8
```

HLS is the compatibility path for most browsers. Use Shaka Player for playback.

## WebRTC low-latency playback

Example WebRTC/WHEP URL:

```text
http://your-server:8889/live/{stream_key}
```

WebRTC is the low-latency path and should be marked experimental until hardened.

## DVR

The initial DVR design is a sliding HLS window:

- Retain the most recent N seconds/minutes of live segments (configurable, default 900s).
- Publish a moving `.m3u8` playlist.
- Allow viewers to pause and rewind within the retained window.
- Clean up expired segments automatically.

## OBS Setup Guide

1. Open **OBS Studio**.
2. Go to **Settings → Stream**.
3. Set **Service** to **Custom...**.
4. Set **Server** to: `rtmp://your-server:1935/live`
5. Set **Stream Key** to your stream key (64-character hex string).
6. Click **OK** and then **Start Streaming**.

Your stream should appear at:
- HLS: `http://your-server:8888/live/{stream_key}/index.m3u8`
- WebRTC: `http://your-server:8889/live/{stream_key}`

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MEDIAMTX_HLS_BASE_URL` | `http://localhost:8888` | Base URL for HLS segments |
| `MEDIAMTX_WEBRTC_BASE_URL` | `http://localhost:8889` | Base URL for WebRTC |
| `RTMP_BASE_URL` | `rtmp://localhost:1935` | Base URL for RTMP ingest |
| `LIVE_HOOK_SECRET` | `changeme-live-hook-secret` | Secret shared with MediaMTX for hook auth |

## Stream lifecycle

```
idle ──► live ──► ending ──► ended
  │                     │
  └── (deleted)         └── (deleted)
```

- **idle**: Stream created, waiting for OBS to connect.
- **live**: OBS is pushing, stream is viewable.
- **ending**: Brief transition state when OBS disconnects.
- **ended**: Stream finished, playback of archive may be available.

## Current status

- [x] MediaMTX Docker configuration
- [x] Live stream CRUD API
- [x] Stream key generation with SHA-256 hashing
- [x] MediaMTX webhook integration (auth, on-publish, on-unpublish)
- [x] Live start/end detection
- [x] HLS/WebRTC URL generation
- [x] Frontend live creation page with OBS guide
- [ ] Live DVR playback UI
- [ ] Live chat (WebSocket + Redis pub/sub)
- [ ] Moderation hooks for live chat
- [ ] DVR export to VOD