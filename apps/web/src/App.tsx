import { useEffect, useState } from 'react';
import WatchPage from './WatchPage';
import UploadPage from './UploadPage';
import SearchPage from './SearchPage';
import ChannelPage from './ChannelPage';
import LivePage from './LivePage';
import LiveWatchPage from './LiveWatchPage';
import LoginPage from './LoginPage';
import { AdminShell } from './admin';
import { getCurrentUser, logout, type CurrentUser } from './api/auth';

type Theme = 'dark' | 'light';

type Route =
  | { page: 'home' }
  | { page: 'watch'; videoId: string }
  | { page: 'upload' }
  | { page: 'search' }
  | { page: 'channel'; userId: string }
  | { page: 'live' }
  | { page: 'live-watch'; videoId: string }
  | { page: 'login' }
  | { page: 'admin' };

function getRoute(): Route {
  const path = window.location.pathname;
  if (path.startsWith('/watch/')) return { page: 'watch', videoId: path.replace('/watch/', '') };
  if (path === '/upload') return { page: 'upload' };
  if (path === '/search') return { page: 'search' };
  if (path.startsWith('/channel/')) return { page: 'channel', userId: path.replace('/channel/', '') };
  if (path === '/live') return { page: 'live' };
  if (path.startsWith('/live/watch/')) return { page: 'live-watch', videoId: path.replace('/live/watch/', '') };
  if (path === '/login') return { page: 'login' };
  if (path === '/admin') return { page: 'admin' };
  return { page: 'home' };
}

type HealthResponse = {
  status: 'ok' | 'error';
  checks: Record<string, string>;
};

export default function App() {
  const [route, setRoute] = useState<Route>(getRoute);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [healthError, setHealthError] = useState<string | null>(null);
  const [user, setUser] = useState<CurrentUser | null | undefined>(undefined);
  const [theme, setTheme] = useState<Theme>(() => {
    try {
      return (localStorage.getItem('theme') as Theme) ?? 'dark';
    } catch {
      return 'dark';
    }
  });

  // Apply theme to <html>
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    try { localStorage.setItem('theme', theme); } catch { /* ignore */ }
  }, [theme]);

  // Listen for browser back/forward
  useEffect(() => {
    const onPop = () => setRoute(getRoute());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  // Load current user
  useEffect(() => {
    getCurrentUser()
      .then(setUser)
      .catch(() => setUser(null));
  }, []);

  // Load health on home page
  useEffect(() => {
    if (route.page !== 'home') return;
    let cancelled = false;
    fetch('/healthz')
      .then((r) => {
        if (!r.ok) throw new Error(`Health check failed: ${r.status}`);
        return r.json() as Promise<HealthResponse>;
      })
      .then((data) => { if (!cancelled) setHealth(data); })
      .catch((err) => { if (!cancelled) setHealthError(err instanceof Error ? err.message : String(err)); });
    return () => { cancelled = true; };
  }, [route.page]);

  function navigate(path: string) {
    window.history.pushState({}, '', path);
    setRoute(getRoute());
  }

  function handleAuthSuccess() {
    getCurrentUser().then(setUser).catch(() => setUser(null));
    navigate('/');
  }

  function handleLogout() {
    logout();
    setUser(null);
    navigate('/');
  }

  function toggleTheme() {
    setTheme((t) => (t === 'dark' ? 'light' : 'dark'));
  }

  const header = (
    <Header
      navigate={navigate}
      user={user ?? null}
      theme={theme}
      onToggleTheme={toggleTheme}
      onLogout={handleLogout}
    />
  );

  if (route.page === 'watch') {
    return <WatchPage videoId={route.videoId} />;
  }

  if (route.page === 'login') {
    return (
      <>
        {header}
        <LoginPage onSuccess={handleAuthSuccess} />
      </>
    );
  }

  if (route.page === 'upload') {
    return (
      <>
        {header}
        <UploadPage />
      </>
    );
  }

  if (route.page === 'search') {
    return (
      <>
        {header}
        <SearchPage />
      </>
    );
  }

  if (route.page === 'channel') {
    return (
      <>
        {header}
        <ChannelPage userId={route.userId} />
      </>
    );
  }

  if (route.page === 'live') {
    return (
      <>
        {header}
        <LivePage />
      </>
    );
  }

  if (route.page === 'live-watch') {
    return (
      <>
        {header}
        <LiveWatchPage streamId={route.videoId} />
      </>
    );
  }

  if (route.page === 'admin') {
    if (user === undefined) {
      return <>{header}<main className="app-shell"><p className="muted">Loading…</p></main></>;
    }
    if (!user || (user.role !== 'admin' && user.role !== 'moderator')) {
      return (
        <>
          {header}
          <main className="app-shell">
            <div className="card" style={{ maxWidth: 480, margin: '2rem auto' }}>
              <h2>Access denied</h2>
              <p>You need moderator or admin privileges to view this page.</p>
              <a href="/" className="button" onClick={(e) => { e.preventDefault(); navigate('/'); }}>
                Go home
              </a>
            </div>
          </main>
        </>
      );
    }
    return (
      <>
        {header}
        <AdminShell />
      </>
    );
  }

  // Home page
  return (
    <>
      {header}
      <main className="app-shell">
        <header className="hero">
          <p className="eyebrow">TPT Online Video</p>
          <h1>Open-source distributed video infrastructure</h1>
          <p>
            A YouTube-like platform scaffold with VOD transcoding, HLS playback, live streaming,
            abstracted storage, search, moderation, and self-hosted deployment.
          </p>
        </header>

        <section className="card-grid">
          <article className="card">
            <h2>Backend status</h2>
            {healthError && <p className="error">API health check failed: {healthError}</p>}
            {!healthError && !health && <p className="muted">Loading API health…</p>}
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
            <h2>Platform pillars</h2>
            <ol>
              <li>Resumable uploads and transcoding queue ✓</li>
              <li>FFmpeg HLS renditions and progress tracking ✓</li>
              <li>Shaka Player ABR playback ✓</li>
              <li>Auth, roles, comments, and search UI ✓</li>
              <li>MediaMTX live RTMP/HLS/WebRTC/DVR pipeline ✓</li>
              <li>Full moderation and admin dashboard ✓</li>
              <li>Windows/Linux self-contained installers</li>
            </ol>
          </article>
        </section>
      </main>
    </>
  );
}

type HeaderProps = {
  navigate: (path: string) => void;
  user: CurrentUser | null;
  theme: Theme;
  onToggleTheme: () => void;
  onLogout: () => void;
};

function Header({ navigate, user, theme, onToggleTheme, onLogout }: HeaderProps) {
  function go(e: React.MouseEvent<HTMLAnchorElement>, path: string) {
    e.preventDefault();
    navigate(path);
  }

  return (
    <header className="site-header" role="banner">
      <a href="/" onClick={(e) => go(e, '/')} className="site-logo" aria-label="TPT Online Video home">
        TPT Online Video
      </a>

      <nav className="site-nav" aria-label="Main navigation">
        <a href="/" onClick={(e) => go(e, '/')}>Home</a>
        <a href="/search" onClick={(e) => go(e, '/search')}>Search</a>
        <a href="/live" onClick={(e) => go(e, '/live')}>Go Live</a>
        {user?.role === 'admin' || user?.role === 'moderator' ? (
          <a href="/admin" onClick={(e) => go(e, '/admin')}>Moderation</a>
        ) : null}
      </nav>

      <div className="site-header-actions">
        <button
          type="button"
          className="theme-toggle"
          onClick={onToggleTheme}
          aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
          title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
        >
          {theme === 'dark' ? '☀️' : '🌙'}
        </button>

        {user ? (
          <>
            <a href="/upload" onClick={(e) => go(e, '/upload')} className="button button--sm">
              Upload
            </a>
            <a
              href={`/channel/${user.id}`}
              onClick={(e) => go(e, `/channel/${user.id}`)}
              className="user-avatar-btn"
              aria-label={`Your channel: ${user.display_name}`}
              title={user.display_name}
            >
              {user.display_name.charAt(0).toUpperCase()}
            </a>
            <button type="button" className="button button--sm button--ghost" onClick={onLogout}>
              Sign out
            </button>
          </>
        ) : (
          <a href="/login" onClick={(e) => go(e, '/login')} className="button button--sm">
            Sign in
          </a>
        )}
      </div>
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
