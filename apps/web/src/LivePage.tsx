import { useState, FormEvent, useEffect } from 'react';
import LivePlayer, { LiveStreamInfo } from './LivePlayer';

type LiveStreamResponse = {
  id: string;
  owner_id: string;
  title: string;
  description: string;
  status: string;
  rtmp_url: string;
  hls_url: string;
  webrtc_url: string;
  dvr_enabled: boolean;
  dvr_window_seconds: number;
  started_at: string | null;
  ended_at: string | null;
  created_at: string;
};

type CreateResponse = {
  stream: LiveStreamResponse;
  stream_key: string;
  stream_key_url: string;
};

type AuthResponse = {
  success: boolean;
  data?: { id: string; email: string; display_name: string };
  error?: { code: string; message: string };
};

const API_BASE = '/api/v1';

function getToken(): string | null {
  return localStorage.getItem('token');
}

async function apiFetch(path: string, options: RequestInit = {}): Promise<any> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  const body = await res.json();
  if (!body.success) {
    throw new Error(body.error?.message || `HTTP ${res.status}`);
  }
  return body.data;
}

export default function LivePage() {
  const [user, setUser] = useState<{ id: string; email: string; display_name: string } | null>(null);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [dvr, setDvr] = useState(true);
  const [creating, setCreating] = useState(false);
  const [result, setResult] = useState<CreateResponse | null>(null);
  const [streams, setStreams] = useState<LiveStreamResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [liveStreams, setLiveStreams] = useState<LiveStreamResponse[]>([]);
  const [loadingLive, setLoadingLive] = useState(true);

  // Check auth and load existing streams
  useEffect(() => {
    async function load() {
      try {
        const authData = await apiFetch('/auth/me');
        setUser(authData);
        const myStreams = await apiFetch('/live/streams');
        setStreams(myStreams);
      } catch (err: any) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    }
    load();

    // Load currently live streams
    fetch('/api/v1/live/streams/live')
      .then((r) => r.ok ? r.json() : [])
      .then((data: LiveStreamResponse[]) => {
        setLiveStreams(data);
        setLoadingLive(false);
      })
      .catch(() => setLoadingLive(false));
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;

    setCreating(true);
    setError(null);
    try {
      const data = await apiFetch('/live/streams', {
        method: 'POST',
        body: JSON.stringify({
          title: title.trim(),
          description: description.trim(),
          dvr_enabled: dvr,
        }),
      });
      setResult(data);
      setTitle('');
      setDescription('');
      // Refresh list
      const myStreams = await apiFetch('/live/streams');
      setStreams(myStreams);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(streamId: string) {
    if (!window.confirm('Delete this live stream? This cannot be undone.')) return;
    try {
      await apiFetch(`/live/streams/${streamId}`, { method: 'DELETE' });
      setStreams(streams.filter((s) => s.id !== streamId));
      if (result?.stream.id === streamId) {
        setResult(null);
      }
    } catch (err: any) {
      setError(err.message);
    }
  }

  function copyToClipboard(text: string, label: string) {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(label);
      setTimeout(() => setCopied(null), 2000);
    });
  }

  if (loading) {
    return <main className="app-shell"><p>Loading...</p></main>;
  }

  if (!user) {
    return (
      <main className="app-shell">
        <div className="card" style={{ maxWidth: '480px', margin: '2rem auto' }}>
          <h2>Live Streaming</h2>
          <p>You need to be logged in to create a live stream.</p>
          <p className="error">{error}</p>
        </div>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <h2>Live Streaming</h2>

      {error && <p className="error">{error}</p>}

      {/* Create form */}
      <section className="card" style={{ marginBottom: '2rem' }}>
        <h3>Create a new live stream</h3>
        <form onSubmit={handleCreate} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <div>
            <label htmlFor="title">Title *</label>
            <input
              id="title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
              placeholder="My Live Stream"
              style={{ width: '100%' }}
            />
          </div>
          <div>
            <label htmlFor="description">Description</label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe your stream..."
              rows={3}
              style={{ width: '100%' }}
            />
          </div>
          <div>
            <label>
              <input type="checkbox" checked={dvr} onChange={(e) => setDvr(e.target.checked)} />
              {' '}Enable DVR (rewind/pause in live player)
            </label>
          </div>
          <button type="submit" className="button" disabled={creating || !title.trim()}>
            {creating ? 'Creating...' : 'Create Stream'}
          </button>
        </form>
      </section>

      {/* Stream key display */}
      {result && (
        <section className="card" style={{ marginBottom: '2rem', border: '2px solid #4caf50' }}>
          <h3>Stream Created</h3>
          <p><strong>Title:</strong> {result.stream.title}</p>
          <p><strong>Status:</strong> {result.stream.status}</p>
          <div style={{ marginBottom: '1rem' }}>
            <label><strong>Stream Key (shown once)</strong></label>
            <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
              <code style={{
                flex: 1, padding: '0.5rem', background: '#1e1e1e', color: '#4ec9b0',
                borderRadius: '4px', wordBreak: 'break-all', fontSize: '0.85rem'
              }}>
                {result.stream_key}
              </code>
              <button className="button small" onClick={() => copyToClipboard(result.stream_key, 'key')}>
                {copied === 'key' ? 'Copied!' : 'Copy'}
              </button>
            </div>
          </div>
          <div style={{ marginBottom: '1rem' }}>
            <label><strong>RTMP URL (for OBS)</strong></label>
            <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
              <code style={{
                flex: 1, padding: '0.5rem', background: '#1e1e1e', color: '#4ec9b0',
                borderRadius: '4px', wordBreak: 'break-all', fontSize: '0.85rem'
              }}>
                {result.stream_key_url}
              </code>
              <button className="button small" onClick={() => copyToClipboard(result.stream_key_url, 'rtmp')}>
                {copied === 'rtmp' ? 'Copied!' : 'Copy'}
              </button>
            </div>
          </div>
          {result.stream.hls_url && (
            <p><strong>HLS URL:</strong> <code>{result.stream.hls_url}</code></p>
          )}
          {result.stream.webrtc_url && (
            <p><strong>WebRTC URL:</strong> <code>{result.stream.webrtc_url}</code></p>
          )}
          <details style={{ marginTop: '1rem' }}>
            <summary>OBS Setup Guide</summary>
            <ol style={{ paddingLeft: '1.5rem' }}>
              <li>Open OBS Studio</li>
              <li>Go to <strong>Settings → Stream</strong></li>
              <li>Set <strong>Service</strong> to <em>Custom...</em></li>
              <li>Set <strong>Server</strong> to: <code>{result.stream_key_url.replace('/' + result.stream_key, '')}</code></li>
              <li>Set <strong>Stream Key</strong> to: <code>{result.stream_key}</code></li>
              <li>Click <strong>OK</strong> and then <strong>Start Streaming</strong></li>
            </ol>
          </details>
        </section>
      )}

      {/* Stream list */}
      <section>
        <h3>Your Streams</h3>
        {streams.length === 0 ? (
          <p>No streams yet. Create one above to get started.</p>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ textAlign: 'left', borderBottom: '1px solid #444' }}>
                <th style={{ padding: '0.5rem' }}>Title</th>
                <th style={{ padding: '0.5rem' }}>Status</th>
                <th style={{ padding: '0.5rem' }}>Created</th>
                <th style={{ padding: '0.5rem' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {streams.map((s) => (
                <tr key={s.id} style={{ borderBottom: '1px solid #333' }}>
                  <td style={{ padding: '0.5rem' }}>{s.title}</td>
                  <td style={{ padding: '0.5rem' }}>
                    <span className={`status-${s.status}`}>{s.status}</span>
                  </td>
                  <td style={{ padding: '0.5rem' }}>
                    {new Date(s.created_at).toLocaleDateString()}
                  </td>
                <td style={{ padding: '0.5rem' }}>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    {s.status === 'live' && (
                      <a
                        href={`/live/watch/${s.id}`}
                        onClick={(e) => {
                          e.preventDefault();
                          window.history.pushState({}, '', `/live/watch/${s.id}`);
                          window.dispatchEvent(new PopStateEvent('popstate'));
                        }}
                        className="button small"
                      >
                        Watch
                      </a>
                    )}
                    <button className="button small" onClick={() => handleDelete(s.id)}>
                      Delete
                    </button>
                  </div>
                </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      {/* Currently live streams (public) */}
      <section style={{ marginTop: '2rem' }}>
        <h3>Currently Live</h3>
        {loadingLive ? (
          <p>Loading live streams...</p>
        ) : liveStreams.length === 0 ? (
          <p>No streams are currently live.</p>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ textAlign: 'left', borderBottom: '1px solid #444' }}>
                <th style={{ padding: '0.5rem' }}>Title</th>
                <th style={{ padding: '0.5rem' }}>Status</th>
                <th style={{ padding: '0.5rem' }}>Started</th>
                <th style={{ padding: '0.5rem' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {liveStreams.map((s) => (
                <tr key={s.id} style={{ borderBottom: '1px solid #333' }}>
                  <td style={{ padding: '0.5rem' }}>{s.title}</td>
                  <td style={{ padding: '0.5rem' }}>
                    <span className="live-badge live-badge--active">LIVE</span>
                  </td>
                  <td style={{ padding: '0.5rem' }}>
                    {s.started_at ? new Date(s.started_at).toLocaleTimeString() : '—'}
                  </td>
                  <td style={{ padding: '0.5rem' }}>
                    <a
                      href={`/live/watch/${s.id}`}
                      onClick={(e) => {
                        e.preventDefault();
                        window.history.pushState({}, '', `/live/watch/${s.id}`);
                        window.dispatchEvent(new PopStateEvent('popstate'));
                      }}
                      className="button small"
                    >
                      Watch
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </main>
  );
}
