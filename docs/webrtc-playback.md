# WebRTC Playback (WHEP)

TPT Online Video supports low-latency WebRTC playback via the WHEP (WebRTC HTTP Egress Protocol) endpoint exposed by MediaMTX. This mode is **experimental** — HLS is the stable playback path.

## How it works

1. The viewer's browser sends an SDP offer (receive-only) to the MediaMTX WHEP endpoint.
2. MediaMTX responds with an SDP answer.
3. ICE negotiation completes and the browser plays the live stream directly over WebRTC.

Expected latency: **~500 ms–2 s** (vs. 15–30 s for HLS).

## URL format

The WHEP endpoint is returned in the `webrtc_url` field of the stream metadata API:

```
GET /api/v1/live/streams/{streamID}
→ { "webrtc_url": "http://<host>:8889/live/<streamKey>", ... }
```

MediaMTX listens for WHEP `POST` requests at that URL.

## Configuration

MediaMTX WebRTC is enabled in `infra/docker/mediamtx/mediamtx.yml`:

```yaml
webRTC: true
webRTCAddress: :8889
webRTCAllowOrigin: "*"
```

For production, restrict `webRTCAllowOrigin` to your domain and ensure port 8889 is externally reachable (or proxied via nginx/Caddy with WebSocket support disabled — WHEP uses plain HTTP POST, not WebSockets).

## Switching modes in the player

The player defaults to HLS. Click the **WebRTC EXP** button in the top-right corner to switch. The player falls back to HLS automatically if WebRTC negotiation fails.

## Troubleshooting

### "WHEP negotiation failed: HTTP 404"

- The stream is not currently live. WebRTC requires an active publisher.
- The `webrtc_url` path does not match the active MediaMTX path. Check that the stream key was not rotated.

### "WHEP: timed out waiting for media track"

- ICE negotiation stalled. Common causes:
  - Firewall blocking UDP ports (MediaMTX uses dynamic UDP for RTP).
  - Missing STUN reachability. The client uses `stun.l.google.com:19302` — ensure outbound UDP 19302 is allowed.
  - Browser WebRTC is disabled (some corporate proxies block it).
- Try switching to HLS as a fallback.

### "WHEP negotiation failed: HTTP 403"

- CORS is blocking the request. Confirm `webRTCAllowOrigin` in `mediamtx.yml` includes your frontend origin.

### No audio/video after successful negotiation

- MediaMTX received no tracks from the publisher (OBS not streaming, or stream key mismatch).
- Check `docker logs mediamtx` for publisher errors.

### High latency in WebRTC mode

- WebRTC RTT is shown in the bottom-right stats bar (WebRTC mode only).
- RTT > 300 ms usually means a relayed (TURN) path is being used. Consider deploying a TURN server co-located with MediaMTX for LAN users.

### Player switches back to HLS unexpectedly

- This is intentional: if `tryWebRTC()` throws (network error, SDP failure, track timeout), the player falls back to HLS silently. Check the browser console for the specific error.

## Known limitations

- No adaptive bitrate in WebRTC mode — viewers receive the source bitrate from OBS.
- TURN server is not bundled. Viewers behind symmetric NAT or strict firewalls will fall back to HLS.
- Audio-only mode is not supported in the current player UI.
- WebRTC DVR (pause/rewind) is not supported; HLS DVR is available when `dvr_enabled: true`.
