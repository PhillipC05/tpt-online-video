import { useEffect, useRef, useState } from 'react';
import shaka from 'shaka-player/dist/shaka-player.ui';

type VideoData = {
  id: string;
  title: string;
  description: string;
  status: string;
  duration_seconds: number | null;
  view_count: number;
  created_at: string;
  hls_manifest_url: string;
  thumbnail_url: string;
  renditions: Array<{ name: string; width: number; height: number; url: string }>;
  owner: { id: string; display_name: string };
};

export default function WatchPage({ videoId }: { videoId: string }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const videoContainerRef = useRef<HTMLDivElement>(null);
  const [video, setVideo] = useState<VideoData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [player, setPlayer] = useState<shaka.Player | null>(null);
  const playerRef = useRef<shaka.Player | null>(null);

  useEffect(() => {
    // Fetch video metadata
    fetch(`/api/v1/videos/${videoId}`)
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

  useEffect(() => {
    if (!video || !videoRef.current || playerRef.current) return;

    // Install Shaka polyfills
    shaka.polyfill.installAll();

    if (!shaka.Player.isBrowserSupported()) {
      setError('Shaka Player is not supported in this browser');
      return;
    }

    const p = new shaka.Player();
    playerRef.current = p;

    p.attach(videoRef.current).then(() => {
      if (video.hls_manifest_url) {
        return p.load(video.hls_manifest_url);
      }
    }).catch((err: unknown) => {
      setError(`Playback error: ${err}`);
    });

    setPlayer(p);

    return () => {
      p.destroy();
      playerRef.current = null;
    };
  }, [video]);

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

  return (
    <main className="app-shell">
      <div className="video-player-wrapper" ref={videoContainerRef}>
        <video
          ref={videoRef}
          className="shaka-video"
          width="100%"
          poster={video.thumbnail_url || undefined}
          controls
          autoPlay
        />
      </div>

      <div className="video-meta" style={{ marginTop: '1rem' }}>
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
        </div>

        {video.owner && (
          <div className="video-owner">
            <strong>{video.owner.display_name}</strong>
          </div>
        )}

        {video.status !== 'ready' && (
          <div className="status-badge">
            Status: {video.status}
            {video.status === 'queued' || video.status === 'transcoding' ? (
              <span> — Processing will complete shortly</span>
            ) : null}
          </div>
        )}

        {video.renditions.length > 0 && (
          <details style={{ marginTop: '1rem' }}>
            <summary>Available qualities</summary>
            <ul>
              {video.renditions.map((r) => (
                <li key={r.name}>
                  {r.name} ({r.width}x{r.height})
                </li>
              ))}
            </ul>
          </details>
        )}
      </div>
    </main>
  );
}

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);

  if (h > 0) {
    return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  }
  return `${m}:${s.toString().padStart(2, '0')}`;
}

export function WatchLink({ videoId }: { videoId: string }) {
  return <a href={`/watch/${videoId}`}>Watch</a>;
}