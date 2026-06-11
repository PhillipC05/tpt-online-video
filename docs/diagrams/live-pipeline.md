# Live Pipeline

```mermaid
sequenceDiagram
    participant Broadcaster
    participant OBS
    participant MediaMTX
    participant API
    participant DB
    participant Redis
    participant Viewer
    participant Browser

    Broadcaster->>API: Create live stream
    API->>DB: Insert live stream (status: waiting)
    API-->>Broadcaster: RTMP URL + stream key

    Broadcaster->>OBS: Configure RTMP
    OBS->>MediaMTX: RTMP publish (stream key)

    MediaMTX->>API: Webhook: stream started
    API->>DB: Update live stream (status: live)
    API->>Redis: Publish stream status event

    Viewer->>Browser: Open watch page
    Browser->>API: Get live stream details
    API-->>Browser: HLS URL, WebRTC URL, status

    Browser->>MediaMTX: HLS playback
    Browser->>MediaMTX: WebRTC/WHEP playback (optional)

    Broadcaster->>OBS: Stop stream
    OBS->>MediaMTX: RTMP disconnect

    MediaMTX->>API: Webhook: stream ended
    API->>DB: Update live stream (status: ended, duration)
    API->>DB: Process DVR segment retention
    API->>Redis: Publish stream end event