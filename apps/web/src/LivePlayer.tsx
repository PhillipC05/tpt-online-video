import { useEffect, useRef, useState, useCallback } from 'react';
import shaka from 'shaka-player/dist/shaka-player.ui';
import { connectWhep, WhepConnection } from './WhepClient';

// ─── Types ─────────────────────────────────────────────────────────────────

export type LiveStreamInfo = {
  id: string;
  title: string;
  description: string;
  status: string;
  hls_url: string;
  webrtc_url: string;
  dvr_enabled: boolean;
  dvr_window_seconds: number;
  started_at: string | null;
  owner?: { id: string; display_name: string };
  viewer_count?: number;
};

type PlayerMode = 'hls' | 'webrtc' | 'none';

interface LivePlayerProps {
  stream: LiveStreamInfo;
  onStreamEnd?: () => void;
}

interface ReconnectConfig {
  maxAttempts: number;
  baseDelayMs: number;
  maxDelayMs: number;
}

const DEFAULT_RECONNECT: ReconnectConfig = {
  maxAttempts: 5,
  baseDelayMs: 1000,
  maxDelayMs: 15000,
};

// How many seconds behind the live edge counts as "at live".
const LIVE_EDGE_TOLERANCE_S = 5;

interface RTCInboundVideoStats {
  roundTripTime?: number;
  jitter?: number;
  packetsLost?: number;
}

// ─── Component ─────────────────────────────────────────────────────────────

export default function LivePlayer({ stream }: LivePlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const playerContainerRef = useRef<HTMLDivElement>(null);

  const [mode, setMode] = useState<PlayerMode>('none');
  const [useWebRTC, setUseWebRTC] = useState(false);

  const [isBuffering, setIsBuffering] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const [reconnecting, setReconnecting] = useState(false);
  const [reconnectAttempt, setReconnectAttempt] = useState(0);

  // DVR state
  const [dvrSeekableS, setDvrSeekableS] = useState(0);   // how far back we can seek
  const [dvrOffsetS, setDvrOffsetS] = useState(0);        // current seconds behind live
  const [atLive, setAtLive] = useState(true);

  // WebRTC stats
  const [rtcStats, setRtcStats] = useState<RTCInboundVideoStats | null>(null);
  const [showDebug, setShowDebug] = useState(false);

  // Refs for cleanup
  const shakaRef = useRef<shaka.Player | null>(null);
  const whepRef = useRef<WhepConnection | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const statsIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const latencyIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const dvrIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const streamEndedRef = useRef(false);

  // ─── Cleanup ────────────────────────────────────────────────────────────

  const cleanup = useCallback(() => {
    if (reconnectTimerRef.current) { clearTimeout(reconnectTimerRef.current); reconnectTimerRef.current = null; }
    if (statsIntervalRef.current) { clearInterval(statsIntervalRef.current); statsIntervalRef.current = null; }
    if (latencyIntervalRef.current) { clearInterval(latencyIntervalRef.current); latencyIntervalRef.current = null; }
    if (dvrIntervalRef.current) { clearInterval(dvrIntervalRef.current); dvrIntervalRef.current = null; }
    if (whepRef.current) { whepRef.current.close(); whepRef.current = null; }
    if (shakaRef.current) { shakaRef.current.destroy(); shakaRef.current = null; }
    setReconnecting(false);
    setReconnectAttempt(0);
  }, []);

  // ─── DVR position polling ────────────────────────────────────────────────

  function startDVRPolling() {
    if (dvrIntervalRef.current) clearInterval(dvrIntervalRef.current);
    dvrIntervalRef.current = setInterval(() => {
      const vid = videoRef.current;
      if (!vid || vid.seekable.length === 0) return;

      const seekEnd = vid.seekable.end(vid.seekable.length - 1);
      const seekStart = vid.seekable.start(0);
      const seekableRange = seekEnd - seekStart;

      setDvrSeekableS(Math.max(0, seekableRange));

      const offset = seekEnd - vid.currentTime;
      setDvrOffsetS(Math.max(0, offset));
      setAtLive(offset < LIVE_EDGE_TOLERANCE_S);
    }, 500);
  }

  // ─── WebRTC playback ─────────────────────────────────────────────────────

  async function tryWebRTC() {
    if (!videoRef.current || !stream.webrtc_url) return false;
    try {
      const wh = await connectWhep(stream.webrtc_url);
      whepRef.current = wh;
      const vid = videoRef.current!;
      vid.srcObject = wh.stream;
      await vid.play();
      setMode('webrtc');

      statsIntervalRef.current = setInterval(async () => {
        if (!whepRef.current) return;
        try {
          const report = await whepRef.current.getStats();
          if (!report) return;
          report.forEach((stat: RTCStats) => {
            if (stat.type === 'inbound-rtp' && (stat as RTCInboundRtpStreamStats).kind === 'video') {
              const s = stat as RTCInboundRtpStreamStats;
              setRtcStats({ roundTripTime: (s as unknown as { roundTripTime?: number }).roundTripTime, jitter: s.jitter, packetsLost: s.packetsLost });
            }
          });
        } catch { /* ignore */ }
      }, 2000);

      return true;
    } catch (err) {
      console.warn('WebRTC failed, falling back to HLS:', err);
      return false;
    }
  }

  // ─── HLS playback (Shaka Player) ─────────────────────────────────────────

  async function tryHLS() {
    if (!videoRef.current || !stream.hls_url) return false;

    shaka.polyfill.installAll();
    if (!shaka.Player.isBrowserSupported()) {
      setError('Shaka Player is not supported in this browser');
      return false;
    }

    try {
      const p = new shaka.Player();
      shakaRef.current = p;

      p.configure({
        streaming: {
          rebufferingGoal: 10,
          bufferingGoal: 30,
          alwaysStreamText: false,
          startAtSegmentBoundary: true,
          liveSync: {
            enabled: true,
            defaultLatency: 30,
            maxLatency: 40,
            playbackRateMin: 0.95,
            playbackRateMax: 1.05,
          },
        },
        manifest: {
          dash: { ignoreDrmInfo: true },
        },
      });

      p.addEventListener('buffering', (e: CustomEvent<{ buffering: boolean }>) => {
        setIsBuffering(e.detail.buffering);
      });

      p.addEventListener('error', (e: CustomEvent<{ code?: number; message?: string } | null>) => {
        const err = e.detail;
        console.error('Shaka error:', err?.code, err?.message);
      });

      await p.attach(videoRef.current);
      await p.load(stream.hls_url);
      setMode('hls');

      latencyIntervalRef.current = setInterval(() => {
        if (!shakaRef.current) return;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const stats = (shakaRef.current as unknown as any).getStats() as shaka.ShakaStats;
        if (stats.liveLatency != null) {
          setLatencyMs(Math.round(stats.liveLatency * 1000));
        }
      }, 2000);

      startDVRPolling();
      return true;
    } catch (err) {
      console.error('HLS playback failed:', err);
      setError(`HLS playback failed: ${err}`);
      return false;
    }
  }

  // ─── Start playback ────────────────────────────────────────────────────

  async function startPlayback(preferWebRTC: boolean) {
    cleanup();
    if (!videoRef.current) return;

    if (preferWebRTC && stream.webrtc_url) {
      const ok = await tryWebRTC();
      if (ok) return;
    }

    if (stream.hls_url) {
      const ok = await tryHLS();
      if (ok) return;
    }

    setError('No playback method succeeded. Check if the stream is live.');
  }

  // ─── Initial load ─────────────────────────────────────────────────────

  useEffect(() => {
    if (stream.status === 'live') {
      startPlayback(false);
    } else {
      setError('Stream is not currently live (status: ' + stream.status + ')');
    }
    return () => { cleanup(); };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stream.id, stream.status]);

  // ─── Stall / reconnect ────────────────────────────────────────────────

  const attemptReconnect = useCallback(() => {
    if (streamEndedRef.current) return;
    setReconnecting(true);
    setReconnectAttempt((prev) => prev + 1);

    const config = DEFAULT_RECONNECT;
    const delay = Math.min(config.baseDelayMs * Math.pow(2, reconnectAttempt), config.maxDelayMs);
    const jitter = delay * (0.5 + Math.random() * 0.5);

    reconnectTimerRef.current = setTimeout(() => {
      startPlayback(useWebRTC);
      setReconnecting(false);
    }, jitter);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reconnectAttempt, useWebRTC, stream]);

  useEffect(() => {
    if (!videoRef.current || mode === 'none') return;
    const vid = videoRef.current;
    let stallTimeout: ReturnType<typeof setTimeout> | null = null;
    let lastTime = vid.currentTime;
    let stalled = 0;

    function onTimeUpdate() {
      if (vid.currentTime > lastTime) { lastTime = vid.currentTime; stalled = 0; }
    }

    function checkStall() {
      if (stallTimeout) clearTimeout(stallTimeout);
      stallTimeout = setTimeout(() => {
        if (vid.currentTime === lastTime && !vid.paused && !vid.ended) {
          stalled++;
          if (stalled >= 3 && reconnectAttempt < DEFAULT_RECONNECT.maxAttempts) {
            cleanup();
            attemptReconnect();
          }
        }
      }, 5000);
    }

    vid.addEventListener('timeupdate', onTimeUpdate);
    const stallInterval = setInterval(checkStall, 3000);
    return () => {
      vid.removeEventListener('timeupdate', onTimeUpdate);
      clearInterval(stallInterval);
      if (stallTimeout) clearTimeout(stallTimeout);
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, reconnectAttempt]);

  // ─── Controls ─────────────────────────────────────────────────────────

  function toggleWebRTC() {
    setUseWebRTC((prev) => {
      const next = !prev;
      startPlayback(next);
      return next;
    });
  }

  function jumpToLive() {
    const vid = videoRef.current;
    if (!vid || vid.seekable.length === 0) return;
    vid.currentTime = vid.seekable.end(vid.seekable.length - 1);
    vid.play();
  }

  function onDVRSeek(e: React.ChangeEvent<HTMLInputElement>) {
    const vid = videoRef.current;
    if (!vid || vid.seekable.length === 0) return;
    const offsetS = Number(e.target.value);
    const liveEdge = vid.seekable.end(vid.seekable.length - 1);
    vid.currentTime = Math.max(vid.seekable.start(0), liveEdge - offsetS);
    if (vid.paused) vid.play();
  }

  function togglePause() {
    const vid = videoRef.current;
    if (!vid) return;
    if (vid.paused) { vid.play(); } else { vid.pause(); }
  }

  function toggleFullscreen() {
    const container = playerContainerRef.current;
    if (!container) return;
    if (document.fullscreenElement === container) {
      document.exitFullscreen();
    } else {
      container.requestFullscreen();
    }
  }

  // ─── Render helpers ────────────────────────────────────────────────────

  const canReconnect = reconnectAttempt < DEFAULT_RECONNECT.maxAttempts;
  const liveForSeconds = stream.started_at
    ? Math.floor((Date.now() - new Date(stream.started_at).getTime()) / 1000)
    : 0;

  const isDVR = stream.dvr_enabled && mode === 'hls' && dvrSeekableS > 0;
  const isPaused = videoRef.current?.paused ?? false;

  return (
    <div ref={playerContainerRef} className="live-player-shell">
      <video
        ref={videoRef}
        className="live-player-video"
        onClick={togglePause}
        playsInline
        autoPlay
        muted
      />

      {/* Top-left: Live badge + duration */}
      <div className="live-player-top-left">
        <span className={`live-badge ${stream.status === 'live' ? 'live-badge--active' : 'live-badge--ended'}`}>
          {stream.status === 'live' ? '🔴 LIVE' : stream.status.toUpperCase()}
        </span>
        {stream.status === 'live' && liveForSeconds > 0 && (
          <span className="live-player-duration">{formatDuration(liveForSeconds)}</span>
        )}
      </div>

      {/* Top-right: Mode switch + debug */}
      <div className="live-player-top-right">
        {stream.webrtc_url && (
          <button
            className="live-player-mode-btn"
            onClick={toggleWebRTC}
            title={useWebRTC ? 'Switch to HLS' : 'Switch to WebRTC (experimental)'}
          >
            {useWebRTC ? 'HLS' : 'WebRTC'}
            {!useWebRTC && <span className="experimental-badge">EXP</span>}
          </button>
        )}
        <button
          className="live-player-debug-btn"
          onClick={() => setShowDebug((v) => !v)}
          title="Debug info"
        >
          ℹ
        </button>
      </div>

      {/* Overlays */}
      {reconnecting && (
        <div className="live-player-reconnect-overlay">
          <div className="live-player-reconnect-inner">
            <div className="spinner"></div>
            <p>Reconnecting... (attempt {reconnectAttempt}/{DEFAULT_RECONNECT.maxAttempts})</p>
            {!canReconnect && <p>Giving up after {DEFAULT_RECONNECT.maxAttempts} attempts</p>}
          </div>
        </div>
      )}

      {isBuffering && (
        <div className="live-player-buffering">
          <div className="player-buffering-spinner" />
        </div>
      )}

      {error && !reconnecting && (
        <div className="live-player-error-overlay">
          <div className="live-player-error-inner">
            <h3>Playback Error</h3>
            <p>{error}</p>
            <div className="live-player-error-actions">
              <button
                className="button"
                onClick={() => { setError(null); setReconnectAttempt(0); startPlayback(useWebRTC); }}
              >
                Retry
              </button>
              {!useWebRTC && stream.webrtc_url && (
                <button
                  className="button button--secondary"
                  onClick={() => { setError(null); setUseWebRTC(true); startPlayback(true); }}
                >
                  Try WebRTC (experimental)
                </button>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Bottom controls */}
      <div className="live-player-controls">
        {/* DVR seek bar — only shown for HLS with a seekable range */}
        {isDVR && (
          <div className="live-player-dvr-bar">
            <span className="live-player-dvr-time">
              {dvrOffsetS > LIVE_EDGE_TOLERANCE_S ? `−${formatDuration(Math.round(dvrOffsetS))}` : 'Live'}
            </span>
            <input
              type="range"
              className="live-player-dvr-seek"
              min={0}
              max={Math.round(dvrSeekableS)}
              step={1}
              value={Math.round(dvrOffsetS)}
              onChange={onDVRSeek}
              title="Seek within DVR window"
            />
            <span className="live-player-dvr-window">
              −{formatDuration(Math.round(dvrSeekableS))}
            </span>
          </div>
        )}

        <div className="live-player-controls-row">
          <div className="live-player-controls-left">
            {/* Pause/resume */}
            <button
              className="live-player-controls-btn"
              onClick={togglePause}
              title={isPaused ? 'Resume' : stream.dvr_enabled ? 'Pause (DVR)' : 'Pause'}
            >
              {isPaused ? '▶' : '⏸'}
            </button>

            {/* Jump to live — only shown when behind live edge */}
            {isDVR && !atLive && (
              <button
                className="live-player-controls-btn live-player-jump-live"
                onClick={jumpToLive}
                title="Jump to live"
              >
                ● LIVE
              </button>
            )}

            {/* DVR indicator dot when at live edge */}
            {isDVR && atLive && (
              <span className="live-player-at-live" title="You are at the live edge">
                ● LIVE
              </span>
            )}

            <span className="live-player-mode-indicator">
              {mode === 'webrtc' ? 'WebRTC' : mode === 'hls' ? 'HLS' : ''}
              {mode === 'webrtc' && <span className="experimental-badge">EXP</span>}
            </span>
          </div>

          <div className="live-player-controls-right">
            {latencyMs != null && (
              <span className="live-player-latency-badge" title="Estimated latency">
                ⏱ {latencyMs > 1000 ? `${(latencyMs / 1000).toFixed(1)}s` : `${latencyMs}ms`}
              </span>
            )}
            {rtcStats && (
              <span className="live-player-rtc-badge" title="RTT (WebRTC)">
                📶 {Math.round(rtcStats.roundTripTime ?? 0)}ms
              </span>
            )}
            <button className="live-player-controls-btn" onClick={toggleFullscreen} title="Fullscreen">
              ⛶
            </button>
          </div>
        </div>
      </div>

      {/* Debug overlay */}
      {showDebug && (
        <div className="live-player-debug">
          {[
            ['Mode', mode.toUpperCase()],
            ['Status', stream.status],
            ['Latency', latencyMs != null ? `${latencyMs}ms` : '—'],
            ['Behind live', dvrOffsetS > 0 ? `${dvrOffsetS.toFixed(1)}s` : '—'],
            ['DVR seekable', dvrSeekableS > 0 ? `${dvrSeekableS.toFixed(0)}s` : '—'],
            ['At live', atLive ? 'yes' : 'no'],
            ['Buffering', isBuffering ? 'yes' : 'no'],
            ['Reconnects', String(reconnectAttempt)],
          ].map(([label, value]) => (
            <div key={label} className="live-player-debug-row">
              <span className="live-player-debug-label">{label}</span>
              <span style={label === 'Buffering' ? { color: isBuffering ? '#f87171' : '#86efac' } : undefined}>
                {value}
              </span>
            </div>
          ))}
          <div className="live-player-debug-row">
            <span className="live-player-debug-label">HLS URL</span>
            <span style={{ fontSize: '10px', wordBreak: 'break-all' }}>{stream.hls_url}</span>
          </div>
          {rtcStats && (
            <>
              {[
                ['RTC RTT', `${Math.round(rtcStats.roundTripTime ?? 0)}ms`],
                ['RTC Jitter', String(rtcStats.jitter ?? '—')],
                ['Packets Lost', String(rtcStats.packetsLost ?? 0)],
              ].map(([label, value]) => (
                <div key={label} className="live-player-debug-row">
                  <span className="live-player-debug-label">{label}</span>
                  <span>{value}</span>
                </div>
              ))}
            </>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Helpers ────────────────────────────────────────────────────────────────

function formatDuration(seconds: number): string {
  if (!seconds || isNaN(seconds)) return '0:00';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  return `${m}:${s.toString().padStart(2, '0')}`;
}
