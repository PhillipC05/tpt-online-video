# Live DVR

DVR (Digital Video Recording) lets viewers pause, rewind, and seek within the recent live stream history while the broadcast is still active. The window defaults to 15 minutes and is configurable per stream.

## How it works

```
OBS → RTMP → MediaMTX → HLS sliding window (disk) → Shaka Player (browser)
```

MediaMTX maintains a fixed-size sliding HLS playlist. Every segment it generates is written to the local filesystem (`/var/mediamtx/hls/<stream-path>/`). The playlist always holds up to `hlsSegmentCount` segments, automatically evicting the oldest as new ones arrive. This gives viewers a rolling DVR window without any additional stream-processing code.

### Segment maths

| Setting | Value |
|---|---|
| `hlsSegmentDuration` | 2 s |
| `hlsSegmentCount` | 450 |
| **DVR window** | 900 s = 15 min |

The API's `dvr_window_seconds` per-stream value caps the seekable range reported to the frontend; the MediaMTX buffer is always sized to the maximum (15 min) so any stream's window fits.

## Configuration

### `MEDIAMTX_HLS_DIRECTORY` (env var, API service)

Local path where the API expects to find MediaMTX HLS segments. Must match the `hlsDirectory` value in `mediamtx.yml`. Default: `/var/mediamtx/hls`.

### Per-stream DVR settings

Set when creating a stream via `POST /api/v1/live/streams`:

```json
{
  "title": "My stream",
  "dvr_enabled": true,
  "dvr_window_seconds": 600
}
```

`dvr_enabled` defaults to `true`. `dvr_window_seconds` defaults to `900` (15 min). The maximum effective value is `900` (the MediaMTX buffer size).

## API

### `GET /api/v1/live/streams/{streamID}/dvr`

Returns DVR metadata. Optional auth (owner sees additional fields).

**Response:**

```json
{
  "dvr_enabled": true,
  "dvr_window_seconds": 900,
  "seekable_seconds": 742.5,
  "at_live_edge": true,
  "hls_dvr_url": "http://mediamtx:8888/live/stk_xxx/index.m3u8"
}
```

`seekable_seconds` reflects how much of the window is actually available — it grows from 0 at stream start, capped at `dvr_window_seconds`.

## Frontend player

The live player (`LivePlayer.tsx`) provides full DVR controls when the stream is HLS and `dvr_enabled = true`:

- **Seek bar** — Range slider spanning the available DVR window. Drag left to rewind; the label shows the current offset (e.g. `−3:42`).
- **● LIVE button** — Appears when the viewer is more than 5 s behind the live edge. Click to jump back to the live position.
- **● LIVE indicator** — Static red label shown when the viewer is at the live edge.
- **Pause / Resume** — Pausing any live HLS stream causes it to diverge from the live edge; the DVR seek bar activates automatically on resume.

### Live-edge tolerance

`LIVE_EDGE_TOLERANCE_S = 5`. A viewer within 5 s of the live edge is considered "at live" and sees the static indicator rather than the jump button. This avoids false positives from normal HLS latency variation.

## Segment cleanup

The `DVRCleaner` background goroutine (started at API boot) scans ended streams every 5 minutes. Once `ended_at + dvr_window_seconds` has elapsed, the corresponding MediaMTX segment directory is removed from disk and `dvr_cleaned_at` is stamped on the stream row so it is not scanned again.

This gives viewers the full DVR window to rewatch VOD-style content after the broadcast ends, then frees the disk space automatically.

### Database column

`live_streams.dvr_cleaned_at TIMESTAMPTZ` — set by the cleaner, `NULL` while retention is active.

## Docker / infrastructure

`mediamtx.yml` mounts the segment directory:

```yaml
hlsDirectory: /var/mediamtx/hls
```

In `docker-compose.yml` (or your orchestrator), bind-mount the same path into both the `mediamtx` container (write) and the `api` container (delete):

```yaml
services:
  mediamtx:
    volumes:
      - mediamtx-hls:/var/mediamtx/hls
  api:
    volumes:
      - mediamtx-hls:/var/mediamtx/hls
    environment:
      MEDIAMTX_HLS_DIRECTORY: /var/mediamtx/hls

volumes:
  mediamtx-hls:
```

## Roadmap

- **Redis buffer** — Stream segment URLs into Redis so the seek range survives MediaMTX restarts without a shared volume.
- **Object storage** — Upload segments to S3/R2 for unlimited retention and horizontal scaling.
- **Per-stream `hlsSegmentCount`** — Requires dynamic MediaMTX path config (API reload or per-path overrides).
