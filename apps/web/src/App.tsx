import { useEffect, useState } from 'react';
import WatchPage from './WatchPage';
import UploadPage from './UploadPage';
import SearchPage from './SearchPage';
import ChannelPage from './ChannelPage';
import LivePage from './LivePage';
import LiveWatchPage from './LiveWatchPage';
type HealthResponse = {
  status: 'ok' | 'error';
  checks: Record<string, string>;
};

function getRoute(): { page: string; videoId?: string; userId?: string } {
  const path = window.location.pathname;
  if (path.startsWith('/watch/')) {
    return { page: 'watch', videoId: path.replace('/watch/', '') };
  }
  if (path === '/upload') {
    return { page: 'upload' };
  }
  if (path === '/search') {
    return { page: 'search' };
  }
  if (path.startsWith('/channel/')) {
    return { page: 'channel', userId: path.replace('/channel/', '') };
  }
  if (path === '/live') {
    return { page: 'live' };
  }
  if (path.startsWith('/live/watch/')) {
    return { page: 'live-watch', videoId: path.replace('/live/watch/', '') };
  }
  return { page: 'home' };
}

export default function App() {
  const [route, setRoute] = useState(getRoute);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Listen for popstate (browser back/forward)
  useEffect(() => {
    const onPopState = () => setRoute(getRoute());
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  // Load health on home page
  useEffect(() => {
    if (route.page !== 'home') return;

    let cancelled = false;

    async function loadHealth() {
      try {
        const response = await fetch('/healthz');
        if (!response.ok) {
          throw new Error(`Health check failed with status ${response.status}`);
        }
        const data = (await response.json()) as HealthResponse;
        if (!cancelled) {
          setHealth(data);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      }
    }

    loadHealth();
    return () => {
      cancelled = true;
    };
  }, [route.page]);

  function navigate(path: string) {
    window.history.pushState({}, '', path);
    setRoute(getRoute());
  }

  // Route handling
  if (route.page === 'watch' && route.videoId) {
    return <WatchPage videoId={route.videoId} />;
  }

  if (route.page === 'upload') {
    return (
      <>
        <Header navigate={navigate} />
        <UploadPage />
      </>
    );
  }

  if (route.page === 'search') {
    return (
      <>
        <Header navigate={navigate} />
        <SearchPage />
      </>
    );
  }

  if (route.page === 'channel' && route.userId) {
    return (
      <>
        <Header navigate={navigate} />
        <ChannelPage userId={route.userId} />
      </>
    );
  }

  if (route.page === 'live') {
    return (
      <>
        <Header navigate={navigate} />
        <LivePage />
      </>
    );
  }

  if (route.page === 'live-watch' && route.videoId) {
    return (
      <>
        <Header navigate={navigate} />
        <LiveWatchPage streamId={route.videoId} />
      </>
    );
  }

  // Home page
  return (
    <>
      <Header navigate={navigate} />
      <main className="app-shell">
        <header className="hero">
          <p className="eyebrow">TPT Online Video</p>
          <h1>Open-source distributed video infrastructure</h1>
          <p>
            A YouTube-like platform scaffold with VOD transcoding, HLS playback, live streaming,
            abstracted storage, search, moderation, and self-hosted deployment in progress.
          </p>
        </header>

        <section className="card-grid">
          <article className="card">
            <h2>Backend status</h2>
            {error && <p className="error">API health check failed: {error}</p>}
            {!error && !health && <p>Loading API health...</p>}
            {health && (
              <dl>
                <dt>Status</dt>
                <dd>{health.status}</dd>
                {Object.entries(health.checks).map(([key, value]) => (
                  <FragmentRow key={key} label={key} value={value} />
                ))}
              </dl>
            )}
          </article>

          <article className="card">
            <h2>Implemented foundation</h2>
            <ul>
              <li>MIT license and README</li>
              <li>Docker Compose infrastructure</li>
              <li>Go API, worker, and live service skeletons</li>
              <li>PostgreSQL schema and migrations</li>
              <li>Storage abstraction for local/S3/Wasabi</li>
              <li>Search abstraction with PostgreSQL FTS provider</li>
              <li>Argon2id password hashing foundation</li>
            </ul>
          </article>

          <article className="card">
            <h2>Next engineering pillars</h2>
            <ol>
              <li>Resumable uploads and transcoding queue ✓</li>
              <li>FFmpeg HLS renditions and progress tracking ✓</li>
              <li>Shaka Player ABR playback ✓</li>
              <li>Auth, roles, comments, and search UI</li>
              <li>MediaMTX live RTMP/HLS/WebRTC/DVR pipeline</li>
              <li>Full moderation and admin dashboard</li>
              <li>Windows/Linux self-contained installers</li>
            </ol>
          </article>
        </section>
      </main>
    </>
  );
}

function Header({ navigate }: { navigate: (path: string) => void }) {
  return (
    <header style={{ display: 'flex', gap: '1rem', padding: '1rem 2rem', alignItems: 'center' }}>
      <a href="/" onClick={(e) => { e.preventDefault(); navigate('/'); }} style={{ fontWeight: 'bold', textDecoration: 'none', color: 'inherit' }}>
        TPT Online Video
      </a>
      <nav style={{ display: 'flex', gap: '1rem' }}>
        <a href="/" onClick={(e) => { e.preventDefault(); navigate('/'); }}>Home</a>
          <a href="/search" onClick={(e) => { e.preventDefault(); navigate('/search'); }}>Search</a>
          <a href="/live" onClick={(e) => { e.preventDefault(); navigate('/live'); }}>Go Live</a>
        </nav>
        <a href="/upload" onClick={(e) => { e.preventDefault(); navigate('/upload'); }} className="button" style={{ marginLeft: 'auto' }}>
          Upload
        </a>
    </header>
  );
}

function FragmentRow({ label, value }: { label: string; value: string }) {
  return (
    <>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </>
  );
}