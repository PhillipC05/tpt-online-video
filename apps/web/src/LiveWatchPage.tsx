import { useEffect, useState } from 'react';
import LiveChat from './LiveChat';
import LivePlayer, { LiveStreamInfo } from './LivePlayer';

// ─── Auth helpers ──────────────────────────────────────────────────────────

function getStoredToken(): string | null {
  try {
    return localStorage.getItem('token');
  } catch {
    return null;
  }
}

function authHeaders(): Record<string, string> {
  const token = getStoredToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

function getStoredUser(): { id: string; display_name: string } | null {
  try {
    const raw = localStorage.getItem('user');
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

// ─── Component ─────────────────────────────────────────────────────────────

export default function LiveWatchPage({ streamId }: { streamId: string }) {
  const [stream, setStream] = useState<LiveStreamInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [streamEnded, setStreamEnded] = useState(false);
  const currentUser = getStoredUser();

  // Fetch stream metadata
  useEffect(() => {
    let cancelled = false;
    let pollInterval: ReturnType<typeof setInterval> | null = null;

    async function fetchStream() {
      try {
        const res = await fetch(`/api/v1/live/streams/${streamId}`, {
          headers: authHeaders(),
        });
        if (!res.ok) {
          throw new Error(`Stream not found (HTTP ${res.status})`);
        }
        const data = await res.json();
        const streamData: LiveStreamInfo = data.success ? data.data : data;

        if (!cancelled) {
          setStream({
            id: streamData.id,
            title: streamData.title,
            description: streamData.description,
            status: streamData.status,
            hls_url: streamData.hls_url || '',
            webrtc_url: streamData.webrtc_url || '',
            dvr_enabled: streamData.dvr_enabled,
            dvr_window_seconds: streamData.dvr_window_seconds,
            started_at: streamData.started_at,
            owner: streamData.owner,
            viewer_count: streamData.viewer_count,
          });
          setLoading(false);

          // If stream is ended, stop polling
          if (streamData.status === 'ended' || streamData.status === 'idle') {
            setStreamEnded(true);
            if (pollInterval) clearInterval(pollInterval);
          }
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
          setLoading(false);
        }
      }
    }

    fetchStream();

    // Poll for status changes if stream is live
    pollInterval = setInterval(() => {
      if (streamEnded) return;
      fetchStream();
    }, 10000); // every 10s

    return () => {
      cancelled = true;
      if (pollInterval) clearInterval(pollInterval);
    };
  }, [streamId]);

  // ─── Render ────────────────────────────────────────────────────────────

  if (loading) {
    return (
      <main className="app-shell">
        <div className="loading-state">
          <div className="spinner" />
          <p>Loading stream...</p>
        </div>
      </main>
    );
  }

  if (error) {
    return (
      <main className="app-shell">
        <div className="card" style={{ maxWidth: '500px', margin: '2rem auto' }}>
          <h2>Stream Error</h2>
          <p className="error">{error}</p>
          <a href="/live" style={{ color: '#60a5fa' }}>Back to live streams</a>
        </div>
      </main>
    );
  }

  if (!stream) {
    return (
      <main className="app-shell">
        <div className="card" style={{ maxWidth: '500px', margin: '2rem auto' }}>
          <h2>Stream Not Found</h2>
          <p>The live stream you're looking for doesn't exist.</p>
          <a href="/live" style={{ color: '#60a5fa' }}>Back to live streams</a>
        </div>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <div className="live-watch-layout">
        {/* Player */}
        <div className="live-watch-player">
          <LivePlayer stream={stream} onStreamEnd={() => setStreamEnded(true)} />

          {/* Stream metadata */}
          <div className="live-watch-meta">
            <h2 className="live-watch-title">{stream.title}</h2>
            {stream.description && (
              <p className="live-watch-description">{stream.description}</p>
            )}
            <div className="live-watch-stats">
              <span className={`live-badge ${stream.status === 'live' ? 'live-badge--active' : 'live-badge--ended'}`}>
                {stream.status === 'live' ? '🔴 LIVE' : stream.status.toUpperCase()}
              </span>
              {stream.dvr_enabled && (
                <span className="live-watch-feature-badge" title="DVR (pause/rewind within window)">
                  DVR {stream.dvr_window_seconds}s
                </span>
              )}
              <span className="live-watch-webrtc-badge" title="WebRTC is experimental">
                WebRTC
                <span className="experimental-badge" style={{ marginLeft: '4px' }}>EXP</span>
              </span>
              {stream.viewer_count != null && (
                <span className="live-watch-viewer-count">
                  👁 {stream.viewer_count}
                </span>
              )}
            </div>
          </div>

          {/* Ended notice */}
          {streamEnded && (
            <div className="live-watch-ended-notice">
              <p>This stream has ended.</p>
              <a href="/live" className="button">Browse other streams</a>
            </div>
          )}
        </div>

        {/* Chat sidebar */}
        <LiveChat
          streamId={streamId}
          currentUserId={currentUser?.id}
          currentDisplayName={currentUser?.display_name}
          isOwner={stream.owner?.id === currentUser?.id}
        />
      </div>
    </main>
  );
}