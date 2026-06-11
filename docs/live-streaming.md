# Live Streaming

TPT Online Video is designed to support:

- RTMP ingest from OBS
- HLS live playback
- WebRTC low-latency playback
- Live DVR rewind/pause
- Real-time live chat

## Recommended media server

The current plan uses **MediaMTX** because it supports RTMP, HLS, WebRTC, and WHEP in a single cross-platform service.

## RTMP ingest

Example OBS stream URL:

```text
rtmp://localhost:1935/live/{stream_key}
```

The API should store only a hash of the stream key.

## HLS live playback

Example HLS URL:

```text
http://localhost:8888/live/{stream_key}/index.m3u8
```

HLS is the compatibility path for most browsers.

## WebRTC low-latency playback

Example WebRTC/WHEP URL:

```text
http://localhost:8889/live/{stream_key}
```

WebRTC is the low-latency path and should be marked experimental until hardened.

## DVR

The initial DVR design is a sliding HLS window:

- Retain the most recent N seconds/minutes of live segments.
- Publish a moving `.m3u8` playlist.
- Allow viewers to pause and rewind within the retained window.
- Clean up expired segments automatically.

## Live chat

Live chat should use:

- WebSocket rooms keyed by live stream ID
- Redis pub/sub for fanout
- PostgreSQL persistence
- Moderation hooks for deletion, timeout, ban, and lock-chat actions

## Current status

The repository currently includes MediaMTX Docker configuration and a live helper skeleton. Full stream lifecycle, DVR, and chat integration remain future tasks.