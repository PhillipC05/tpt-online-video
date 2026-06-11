import { useCallback, useEffect, useRef, useState } from 'react';
import shaka from 'shaka-player/dist/shaka-player.ui';

// shaka-player's clutz-generated d.ts doesn't expose Player methods or the
// shaka.extern namespace through a bundler-mode import, so we define minimal
// local types and cast to any where needed.
type ShakaTrack = {
  id: number;
  bandwidth: number;
  width?: number;
  height?: number;
  active: boolean;
};

// ─── Data types ───────────────────────────────────────────────────────────────

type SubtitleTrack = {
  language: string;
  label: string;
  url: string;
};

type VideoData = {
  id: string;
  title: string;
  description: string;
  visibility: string;
  status: string;
  duration_seconds: number | null;
  view_count: number;
  created_at: string;
  hls_manifest_url: string;
  dash_manifest_url?: string;
  thumbnail_url: string;
  renditions: Array<{ name: string; width: number; height: number; url: string }>;
  subtitle_tracks?: SubtitleTrack[];
  owner: { id: string; display_name: string };
};

type RelatedVideo = {
  id: string;
  title: string;
  duration_seconds: number | null;
  view_count: number;
  created_at: string;
  thumbnail_url: string;
  owner_name: string;
};

type PlayerMetrics = {
  estimatedBandwidth: number;
  streamBandwidth: number;
  currentWidth: number;
  currentHeight: number;
  bufferAhead: number;
  stallsDetected: number;
};

type QualitySwitch = {
  time: Date;
  height: number;
  bandwidth: number;
  fromAdaptation: boolean;
};

type PlayerErrorEntry = {
  time: Date;
  code: number;
  message: string;
};

// ─── Auth helpers ─────────────────────────────────────────────────────────────

function getStoredToken(): string | null {
  try {
    return localStorage.getItem('access_token');
  } catch {
    return null;
  }
}

function authHeaders(): Record<string, string> {
  const token = getStoredToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// ─── Main component ───────────────────────────────────────────────────────────

export default function WatchPage({ videoId }: { videoId: string }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const playerContainerRef = useRef<HTMLDivElement>(null);

  const [video, setVideo] = useState<VideoData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);
  const [related, setRelated] = useState<RelatedVideo[]>([]);

  const playerRef = useRef<shaka.Player | null>(null);

  // Player controls state
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [controlsVisible, setControlsVisible] = useState(true);
  const hideTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Metrics & events
  const [isBuffering, setIsBuffering] = useState(false);
  const [metrics, setMetrics] = useState<PlayerMetrics>({
    estimatedBandwidth: 0,
    streamBandwidth: 0,
    currentWidth: 0,
    currentHeight: 0,
    bufferAhead: 0,
    stallsDetected: 0,
  });
  const [qualitySwitches, setQualitySwitches] = useState<QualitySwitch[]>([]);
  const [playerErrors, setPlayerErrors] = useState<PlayerErrorEntry[]>([]);
  const [showDebug, setShowDebug] = useState(false);

  // Quality tracks
  const [tracks, setTracks] = useState<ShakaTrack[]>([]);
  const [abrEnabled, setAbrEnabled] = useState(true);
  const [activeTrackId, setActiveTrackId] = useState<number | null>(null);

  // Captions
  const [ccEnabled, setCcEnabled] = useState(false);
  const [hasSubtitles, setHasSubtitles] = useState(false);

  // Fetch current user
  useEffect(() => {
    const token = getStoredToken();
    if (!token) return;
    fetch('/api/v1/auth/me', { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: { id?: string } | null) => {
        if (data?.id) setCurrentUserId(data.id);
      })
      .catch(() => {});
  }, []);

  // Fetch video metadata
  useEffect(() => {
    fetch(`/api/v1/videos/${videoId}`, { headers: authHeaders() })
      .then((res) => {
        if (!res.ok) throw new Error('Video not found');
        return res.json();
      })
      .then((data: VideoData) => {
        setVideo(data);
        setLoading(false);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : String(err));
        setLoading(false);
      });
  }, [videoId]);

  // Fetch related videos
  useEffect(() => {
    fetch(`/api/v1/videos/${videoId}/related`)
      .then((r) => (r.ok ? r.json() : []))
      .then((data: RelatedVideo[]) => setRelated(data))
      .catch(() => {});
  }, [videoId]);

  // Controls auto-hide
  const revealControls = useCallback(() => {
    setControlsVisible(true);
    if (hideTimer.current) clearTimeout(hideTimer.current);
    hideTimer.current = setTimeout(() => {
      if (!videoRef.current?.paused) setControlsVisible(false);
    }, 3000);
  }, []);

  // Initialise Shaka + wire up all events and metrics polling
  useEffect(() => {
    if (!video || !videoRef.current || playerRef.current) return;

    shaka.polyfill.installAll();
    if (!shaka.Player.isBrowserSupported()) {
      setError('Shaka Player is not supported in this browser');
      return;
    }

    const p = new shaka.Player();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const pa = p as any; // clutz-generated d.ts doesn't expose Player methods under bundler moduleResolution
    playerRef.current = p;

    // ── Shaka events ──────────────────────────────────────────────────────────

    pa.addEventListener('buffering', (e: CustomEvent<{ buffering: boolean }>) => {
      setIsBuffering(e.detail.buffering);
    });

    pa.addEventListener('adaptation', (e: CustomEvent<{ newTrack: ShakaTrack }>) => {
      const track = e.detail.newTrack;
      setActiveTrackId(track.id);
      setQualitySwitches((prev) =>
        [...prev.slice(-19), {
          time: new Date(),
          height: track.height ?? 0,
          bandwidth: track.bandwidth,
          fromAdaptation: true,
        }]
      );
    });

    pa.addEventListener('variantchanged', (e: CustomEvent<{ newTrack: ShakaTrack }>) => {
      if (e.detail.newTrack) setActiveTrackId(e.detail.newTrack.id);
    });

    pa.addEventListener('trackschanged', () => {
      const variantTracks = (pa.getVariantTracks() as ShakaTrack[]).sort(
        (a, b) => (b.height ?? 0) - (a.height ?? 0)
      );
      setTracks(variantTracks);
      const active = variantTracks.find((t) => t.active);
      if (active) setActiveTrackId(active.id);
    });

    pa.addEventListener('error', (e: CustomEvent<{ code?: number; message?: string } | null>) => {
      const err = e.detail;
      const code = err?.code ?? 0;
      const message = err?.message ?? 'Unknown player error';
      setPlayerErrors((prev) =>
        [...prev.slice(-19), { time: new Date(), code, message }]
      );
      setError(`Player error (${code}): ${message}`);
    });

    // ── Attach + load ─────────────────────────────────────────────────────────

    // Prefer DASH when available — Shaka was built for it.
    const manifestUrl = video.dash_manifest_url || video.hls_manifest_url;

    p.attach(videoRef.current)
      .then(() => {
        if (manifestUrl) return pa.load(manifestUrl);
      })
      .then(() => {
        // Add any subtitle tracks supplied by the API.
        if (video.subtitle_tracks?.length) {
          setHasSubtitles(true);
          return Promise.all(
            video.subtitle_tracks.map((t) =>
              pa.addTextTrackAsync(t.url, t.language, 'subtitle', 'text/vtt', undefined, t.label)
                .catch(() => {})
            )
          );
        }
      })
      .catch((err: unknown) => {
        setError(`Playback error: ${err}`);
      });

    // ── Video element events ──────────────────────────────────────────────────

    const vid = videoRef.current;

    function onPlay() { setIsPlaying(true); revealControls(); }
    function onPause() { setIsPlaying(false); setControlsVisible(true); if (hideTimer.current) clearTimeout(hideTimer.current); }
    function onTimeUpdate() { setCurrentTime(vid.currentTime); }
    function onDurationChange() { setDuration(vid.duration || 0); }
    function onVolumeChange() { setVolume(vid.volume); setIsMuted(vid.muted); }
    function onEnded() { setIsPlaying(false); setControlsVisible(true); }

    vid.addEventListener('play', onPlay);
    vid.addEventListener('pause', onPause);
    vid.addEventListener('timeupdate', onTimeUpdate);
    vid.addEventListener('durationchange', onDurationChange);
    vid.addEventListener('volumechange', onVolumeChange);
    vid.addEventListener('ended', onEnded);

    // ── Metrics polling ───────────────────────────────────────────────────────

    const metricsInterval = setInterval(() => {
      if (!playerRef.current || !videoRef.current) return;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const pr = playerRef.current as any;
      const stats = pr.getStats() as { estimatedBandwidth: number; streamBandwidth: number; width: number; height: number; stallsDetected: number };
      const buffered = pr.getBufferedInfo() as { total: Array<{ start: number; end: number }> };
      const t = videoRef.current.currentTime;
      let bufferAhead = 0;
      for (const range of buffered.total) {
        if (range.start <= t && t <= range.end) {
          bufferAhead = range.end - t;
          break;
        }
      }
      setMetrics({
        estimatedBandwidth: stats.estimatedBandwidth,
        streamBandwidth: stats.streamBandwidth,
        currentWidth: stats.width,
        currentHeight: stats.height,
        bufferAhead,
        stallsDetected: stats.stallsDetected,
      });
    }, 1000);

    // ── Fullscreen change ─────────────────────────────────────────────────────

    function onFullscreenChange() {
      setIsFullscreen(document.fullscreenElement === playerContainerRef.current);
    }
    document.addEventListener('fullscreenchange', onFullscreenChange);

    return () => {
      clearInterval(metricsInterval);
      vid.removeEventListener('play', onPlay);
      vid.removeEventListener('pause', onPause);
      vid.removeEventListener('timeupdate', onTimeUpdate);
      vid.removeEventListener('durationchange', onDurationChange);
      vid.removeEventListener('volumechange', onVolumeChange);
      vid.removeEventListener('ended', onEnded);
      document.removeEventListener('fullscreenchange', onFullscreenChange);
      p.destroy();
      playerRef.current = null;
    };
  }, [video, revealControls]);

  // ── Keyboard shortcuts ────────────────────────────────────────────────────

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      const vid = videoRef.current;
      if (!vid || !playerRef.current) return;
      // Don't intercept when typing in inputs
      const tag = (e.target as HTMLElement).tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

      switch (e.key) {
        case ' ':
        case 'k':
          e.preventDefault();
          vid.paused ? void vid.play() : vid.pause();
          break;
        case 'ArrowLeft':
          e.preventDefault();
          vid.currentTime = Math.max(0, vid.currentTime - 10);
          revealControls();
          break;
        case 'ArrowRight':
          e.preventDefault();
          vid.currentTime = Math.min(vid.duration || 0, vid.currentTime + 10);
          revealControls();
          break;
        case 'ArrowUp':
          e.preventDefault();
          vid.volume = Math.min(1, vid.volume + 0.1);
          revealControls();
          break;
        case 'ArrowDown':
          e.preventDefault();
          vid.volume = Math.max(0, vid.volume - 0.1);
          revealControls();
          break;
        case 'm':
        case 'M':
          e.preventDefault();
          vid.muted = !vid.muted;
          break;
        case 'f':
        case 'F':
          e.preventDefault();
          toggleFullscreen();
          break;
        case 'c':
        case 'C':
          e.preventDefault();
          setCcEnabled((v) => {
            const next = !v;
            (playerRef.current as any)?.setTextTrackVisibility(next);
            return next;
          });
          break;
        case 'd':
        case 'D':
          e.preventDefault();
          setShowDebug((v) => !v);
          break;
        default:
          if (e.key >= '0' && e.key <= '9') {
            e.preventDefault();
            const pct = parseInt(e.key, 10) / 10;
            vid.currentTime = (vid.duration || 0) * pct;
            revealControls();
          }
      }
    }
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [revealControls]);

  // ── Player control handlers ───────────────────────────────────────────────

  function togglePlayPause() {
    const vid = videoRef.current;
    if (!vid) return;
    vid.paused ? void vid.play() : vid.pause();
  }

  function handleSeek(e: React.ChangeEvent<HTMLInputElement>) {
    const vid = videoRef.current;
    if (!vid) return;
    vid.currentTime = Number(e.target.value);
  }

  function handleVolume(e: React.ChangeEvent<HTMLInputElement>) {
    const vid = videoRef.current;
    if (!vid) return;
    const v = Number(e.target.value);
    vid.volume = v;
    vid.muted = v === 0;
  }

  function toggleMute() {
    const vid = videoRef.current;
    if (!vid) return;
    vid.muted = !vid.muted;
  }

  function toggleFullscreen() {
    const container = playerContainerRef.current;
    if (!container) return;
    if (document.fullscreenElement === container) {
      void document.exitFullscreen();
    } else {
      void container.requestFullscreen();
    }
  }

  function toggleCC() {
    setCcEnabled((v) => {
      const next = !v;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (playerRef.current as any)?.setTextTrackVisibility(next);
      return next;
    });
  }

  function selectTrack(trackId: number | null) {
    const p = playerRef.current;
    if (!p) return;
    if (trackId === null) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (p as any).configure({ abr: { enabled: true } });
      setAbrEnabled(true);
    } else {
      const track = tracks.find((t) => t.id === trackId);
      if (!track) return;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (p as any).configure({ abr: { enabled: false } });
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (p as any).selectVariantTrack(track, false);
      setAbrEnabled(false);
      setActiveTrackId(trackId);
      setQualitySwitches((prev) =>
        [...prev.slice(-19), {
          time: new Date(),
          height: track.height ?? 0,
          bandwidth: track.bandwidth,
          fromAdaptation: false,
        }]
      );
    }
  }

  // ── Render ────────────────────────────────────────────────────────────────

  if (loading) {
    return (
      <main className="app-shell">
        <p>Loading video...</p>
      </main>
    );
  }

  if (error) {
    return (
      <main className="app-shell">
        <div className="card">
          <h2>Error</h2>
          <p className="error">{error}</p>
          <a href="/">Back to home</a>
        </div>
      </main>
    );
  }

  if (!video) {
    return (
      <main className="app-shell">
        <p>Video not found.</p>
        <a href="/">Back to home</a>
      </main>
    );
  }

  const isOwner = currentUserId !== null && currentUserId === video.owner.id;
  const activeTrack = tracks.find((t) => t.id === activeTrackId);

  return (
    <main className="app-shell">
      <div style={{ display: 'flex', gap: '2rem', alignItems: 'flex-start' }}>

        {/* Main column */}
        <div style={{ flex: '1 1 0', minWidth: 0 }}>

          {/* Player shell */}
          <div
            ref={playerContainerRef}
            className="player-shell"
            onMouseMove={revealControls}
            onMouseLeave={() => {
              if (!videoRef.current?.paused) setControlsVisible(false);
            }}
            style={{ cursor: controlsVisible ? 'default' : 'none' }}
          >
            <video
              ref={videoRef}
              className="player-video"
              poster={video.thumbnail_url || undefined}
              autoPlay
              onClick={togglePlayPause}
            />

            {/* Buffering spinner */}
            {isBuffering && (
              <div className="player-buffering">
                <div className="player-buffering-spinner" />
              </div>
            )}

            {/* Debug / demo overlay */}
            {showDebug && (
              <div className="player-debug">
                <div className="player-debug-row">
                  <span className="player-debug-label">Resolution</span>
                  <span>{metrics.currentWidth && metrics.currentHeight
                    ? `${metrics.currentWidth}×${metrics.currentHeight}`
                    : '—'}</span>
                </div>
                <div className="player-debug-row">
                  <span className="player-debug-label">Stream</span>
                  <span>{formatBitrate(metrics.streamBandwidth)}</span>
                </div>
                <div className="player-debug-row">
                  <span className="player-debug-label">Bandwidth</span>
                  <span>{formatBitrate(metrics.estimatedBandwidth)}</span>
                </div>
                <div className="player-debug-row">
                  <span className="player-debug-label">Buffer</span>
                  <span>{metrics.bufferAhead.toFixed(1)} s ahead</span>
                </div>
                <div className="player-debug-row">
                  <span className="player-debug-label">Stalls</span>
                  <span>{metrics.stallsDetected}</span>
                </div>
                <div className="player-debug-row">
                  <span className="player-debug-label">Buffering</span>
                  <span style={{ color: isBuffering ? '#f87171' : '#86efac' }}>
                    {isBuffering ? 'yes' : 'no'}
                  </span>
                </div>
                <div className="player-debug-row">
                  <span className="player-debug-label">ABR</span>
                  <span>{abrEnabled ? 'auto' : 'manual'}</span>
                </div>
                <div className="player-debug-row">
                  <span className="player-debug-label">Switches</span>
                  <span>{qualitySwitches.length}</span>
                </div>
                {playerErrors.length > 0 && (
                  <div className="player-debug-row" style={{ color: '#f87171' }}>
                    <span className="player-debug-label">Last error</span>
                    <span>{playerErrors[playerErrors.length - 1].code}</span>
                  </div>
                )}
              </div>
            )}

            {/* Custom controls */}
            <div className={`player-controls${controlsVisible ? '' : ' player-controls--hidden'}`}>

              {/* Seek bar */}
              <input
                type="range"
                className="player-seek"
                min={0}
                max={duration || 1}
                step={0.25}
                value={currentTime}
                onChange={handleSeek}
                aria-label="Seek"
              />

              {/* Controls row */}
              <div className="player-controls-row">
                {/* Left group */}
                <div className="player-controls-group">
                  <button
                    className="player-btn"
                    onClick={togglePlayPause}
                    aria-label={isPlaying ? 'Pause' : 'Play'}
                    title={isPlaying ? 'Pause (Space)' : 'Play (Space)'}
                  >
                    {isPlaying ? '⏸' : '▶'}
                  </button>

                  <div className="player-volume-wrap">
                    <button
                      className="player-btn"
                      onClick={toggleMute}
                      aria-label={isMuted || volume === 0 ? 'Unmute' : 'Mute'}
                      title="Mute (M)"
                    >
                      {isMuted || volume === 0 ? '🔇' : volume < 0.5 ? '🔉' : '🔊'}
                    </button>
                    <input
                      type="range"
                      className="player-volume"
                      min={0}
                      max={1}
                      step={0.05}
                      value={isMuted ? 0 : volume}
                      onChange={handleVolume}
                      aria-label="Volume"
                    />
                  </div>

                  <span className="player-time">
                    {formatDuration(currentTime)} / {formatDuration(duration)}
                  </span>
                </div>

                {/* Right group */}
                <div className="player-controls-group">
                  {/* Active rendition badge */}
                  {activeTrack && (
                    <span className="player-quality-badge">
                      {activeTrack.height ? `${activeTrack.height}p` : ''}
                      {!abrEnabled && ' ·  manual'}
                    </span>
                  )}

                  {/* Quality selector */}
                  {tracks.length > 0 && (
                    <select
                      className="player-quality-select"
                      value={abrEnabled ? 'auto' : String(activeTrackId)}
                      onChange={(e) => selectTrack(e.target.value === 'auto' ? null : Number(e.target.value))}
                      aria-label="Quality"
                      title="Quality"
                    >
                      <option value="auto">Auto</option>
                      {tracks.map((t) => (
                        <option key={t.id} value={String(t.id)}>
                          {t.height ? `${t.height}p` : 'unknown'} · {formatBitrate(t.bandwidth)}
                        </option>
                      ))}
                    </select>
                  )}

                  {/* CC toggle — only shown when subtitle tracks are loaded */}
                  {hasSubtitles && (
                    <button
                      className={`player-btn${ccEnabled ? ' player-btn--active' : ''}`}
                      onClick={toggleCC}
                      aria-label={ccEnabled ? 'Disable captions' : 'Enable captions'}
                      title="Captions (C)"
                    >
                      CC
                    </button>
                  )}

                  {/* Debug toggle */}
                  <button
                    className={`player-btn${showDebug ? ' player-btn--active' : ''}`}
                    onClick={() => setShowDebug((v) => !v)}
                    aria-label="Debug overlay"
                    title="Debug overlay (D)"
                  >
                    ℹ
                  </button>

                  {/* Fullscreen */}
                  <button
                    className="player-btn"
                    onClick={toggleFullscreen}
                    aria-label={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
                    title="Fullscreen (F)"
                  >
                    {isFullscreen ? '⛶' : '⛶'}
                  </button>
                </div>
              </div>
            </div>
          </div>

          {/* Keyboard shortcuts hint */}
          <div style={{ fontSize: '0.7rem', color: '#475569', marginTop: '0.4rem', textAlign: 'right' }}>
            Space · ←/→ 10s · ↑/↓ vol · M mute · C captions · F fullscreen · D debug
          </div>

          {/* Video metadata */}
          <div className="video-meta" style={{ marginTop: '0.75rem' }}>
            <h2>{video.title}</h2>
            {video.description && <p>{video.description}</p>}

            <div className="video-stats">
              <span>{video.view_count.toLocaleString()} views</span>
              <span> · </span>
              <span>{new Date(video.created_at).toLocaleDateString()}</span>
              {video.duration_seconds && (
                <>
                  <span> · </span>
                  <span>{formatDuration(video.duration_seconds)}</span>
                </>
              )}
              <span> · </span>
              <span style={{ textTransform: 'capitalize' }}>{video.visibility}</span>
            </div>

            {video.owner && (
              <div className="video-owner">
                <strong>{video.owner.display_name}</strong>
              </div>
            )}

            {video.status !== 'ready' && (
              <div className="status-badge">
                Status: {video.status}
                {(video.status === 'queued' || video.status === 'transcoding') && (
                  <span> — Processing will complete shortly</span>
                )}
              </div>
            )}

            {isOwner && (
              <EditVideoForm
                video={video}
                onSaved={(updated) => setVideo((v) => v ? { ...v, ...updated } : v)}
                onDeleted={() => { window.location.href = '/'; }}
              />
            )}
          </div>
        </div>

        {/* Related videos sidebar */}
        {related.length > 0 && (
          <aside style={{ width: '280px', flexShrink: 0 }}>
            <h3 style={{ marginBottom: '0.75rem' }}>More from this channel</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              {related.map((rv) => (
                <a
                  key={rv.id}
                  href={`/watch/${rv.id}`}
                  style={{ display: 'flex', gap: '0.5rem', textDecoration: 'none', color: 'inherit' }}
                >
                  {rv.thumbnail_url ? (
                    <img
                      src={rv.thumbnail_url}
                      alt={rv.title}
                      style={{ width: '120px', height: '68px', objectFit: 'cover', borderRadius: '4px', flexShrink: 0 }}
                    />
                  ) : (
                    <div style={{ width: '120px', height: '68px', background: '#222', borderRadius: '4px', flexShrink: 0 }} />
                  )}
                  <div>
                    <div style={{ fontWeight: '600', fontSize: '0.875rem', lineHeight: 1.3 }}>{rv.title}</div>
                    <div style={{ fontSize: '0.75rem', color: '#888', marginTop: '0.25rem' }}>{rv.owner_name}</div>
                    <div style={{ fontSize: '0.75rem', color: '#888' }}>
                      {rv.view_count.toLocaleString()} views
                      {rv.duration_seconds ? ` · ${formatDuration(rv.duration_seconds)}` : ''}
                    </div>
                  </div>
                </a>
              ))}
            </div>
          </aside>
        )}
      </div>
    </main>
  );
}

// ─── Edit form ────────────────────────────────────────────────────────────────

type EditFormProps = {
  video: VideoData;
  onSaved: (fields: Partial<VideoData>) => void;
  onDeleted: () => void;
};

function EditVideoForm({ video, onSaved, onDeleted }: EditFormProps) {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState(video.title);
  const [description, setDescription] = useState(video.description);
  const [visibility, setVisibility] = useState(video.visibility);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setErr(null);
    try {
      const res = await fetch(`/api/v1/videos/${video.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ title, description, visibility }),
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to save');
      }
      onSaved({ title, description, visibility });
      setOpen(false);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!window.confirm('Delete this video? This cannot be undone.')) return;
    setDeleting(true);
    setErr(null);
    try {
      const res = await fetch(`/api/v1/videos/${video.id}`, {
        method: 'DELETE',
        headers: authHeaders(),
      });
      if (!res.ok && res.status !== 204) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to delete');
      }
      onDeleted();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setDeleting(false);
    }
  }

  return (
    <div style={{ marginTop: '1.5rem', borderTop: '1px solid #333', paddingTop: '1rem' }}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        style={{ fontSize: '0.875rem', cursor: 'pointer' }}
      >
        {open ? 'Cancel editing' : 'Edit video'}
      </button>

      {open && (
        <form onSubmit={handleSave} style={{ marginTop: '1rem', display: 'flex', flexDirection: 'column', gap: '0.75rem', maxWidth: '480px' }}>
          <div>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.875rem' }}>Title</label>
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
              style={{ width: '100%', padding: '0.4rem 0.6rem', boxSizing: 'border-box' }}
            />
          </div>

          <div>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.875rem' }}>Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
              style={{ width: '100%', padding: '0.4rem 0.6rem', boxSizing: 'border-box', resize: 'vertical' }}
            />
          </div>

          <div>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.875rem' }}>Visibility</label>
            <select value={visibility} onChange={(e) => setVisibility(e.target.value)} style={{ padding: '0.4rem 0.6rem' }}>
              <option value="public">Public</option>
              <option value="unlisted">Unlisted</option>
              <option value="private">Private</option>
            </select>
          </div>

          {err && <p className="error" style={{ margin: 0 }}>{err}</p>}

          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="submit" disabled={saving} className="button">
              {saving ? 'Saving…' : 'Save changes'}
            </button>
            <button
              type="button"
              onClick={handleDelete}
              disabled={deleting}
              style={{ marginLeft: 'auto', color: '#f66', border: '1px solid #f66', background: 'transparent', cursor: 'pointer', padding: '0.4rem 0.8rem' }}
            >
              {deleting ? 'Deleting…' : 'Delete video'}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDuration(seconds: number): string {
  if (!seconds || isNaN(seconds)) return '0:00';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  return `${m}:${s.toString().padStart(2, '0')}`;
}

function formatBitrate(bps: number): string {
  if (!bps) return '—';
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} Mbps`;
  return `${Math.round(bps / 1000)} kbps`;
}

export function WatchLink({ videoId }: { videoId: string }) {
  return <a href={`/watch/${videoId}`}>Watch</a>;
}
