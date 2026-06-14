import { useEffect, useState } from 'react';
import { apiGet, apiPatch, apiDelete } from '../api/client';

type AdminVideo = {
  id: string;
  title: string;
  owner_name: string;
  status: string;
  visibility: string;
  view_count: number;
  created_at: string;
};

export default function VideoManagement() {
  const [videos, setVideos] = useState<AdminVideo[]>([]);
  const [total, setTotal] = useState(0);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [visFilter, setVisFilter] = useState('');
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const limit = 50;

  useEffect(() => { load(); }, [search, statusFilter, visFilter, offset]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (search) params.set('q', search);
      if (statusFilter) params.set('status', statusFilter);
      if (visFilter) params.set('visibility', visFilter);
      params.set('limit', String(limit));
      params.set('offset', String(offset));
      const data = await apiGet<{ videos: AdminVideo[]; total: number }>(
        `/api/v1/admin/videos?${params}`,
      );
      setVideos(data.videos);
      setTotal(data.total);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load videos');
    } finally {
      setLoading(false);
    }
  }

  async function setVisibility(videoId: string, visibility: string) {
    setActionError(null);
    try {
      await apiPatch(`/api/v1/admin/videos/${videoId}`, { visibility });
      load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to update video');
    }
  }

  async function deleteVideo(videoId: string) {
    if (!confirm('Permanently delete this video?')) return;
    setActionError(null);
    try {
      await apiDelete(`/api/v1/admin/videos/${videoId}`);
      load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to delete video');
    }
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 20px', fontSize: '1.25rem' }}>Video Management</h2>

      <div style={{ display: 'flex', gap: 10, marginBottom: 16, flexWrap: 'wrap' }}>
        <input
          style={inputStyle}
          placeholder="Search titles…"
          value={search}
          onChange={e => { setSearch(e.target.value); setOffset(0); }}
        />
        <select style={inputStyle} value={statusFilter} onChange={e => { setStatusFilter(e.target.value); setOffset(0); }}>
          <option value="">All Status</option>
          <option value="ready">Ready</option>
          <option value="transcoding">Transcoding</option>
          <option value="queued">Queued</option>
          <option value="failed">Failed</option>
        </select>
        <select style={inputStyle} value={visFilter} onChange={e => { setVisFilter(e.target.value); setOffset(0); }}>
          <option value="">All Visibility</option>
          <option value="public">Public</option>
          <option value="unlisted">Unlisted</option>
          <option value="private">Private</option>
          <option value="removed">Removed</option>
        </select>
        <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem', alignSelf: 'center' }}>{total} total</span>
      </div>

      {error && <div style={errorBanner}>{error}</div>}
      {actionError && <div style={errorBanner}>{actionError}</div>}
      {loading && <div style={{ color: 'var(--text-muted)', padding: '24px 0' }}>Loading…</div>}

      {!loading && (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Title', 'Owner', 'Status', 'Visibility', 'Views', 'Uploaded', 'Actions'].map(h => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {videos.map(v => (
              <tr key={v.id}>
                <td style={{ ...tdStyle, maxWidth: 260, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {v.title}
                </td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)' }}>{v.owner_name}</td>
                <td style={tdStyle}>
                  <span style={{ ...badge, background: videoStatusColor(v.status) }}>{v.status}</span>
                </td>
                <td style={tdStyle}>
                  <span style={{ ...badge, background: visibilityColor(v.visibility) }}>{v.visibility}</span>
                </td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)' }}>{v.view_count.toLocaleString()}</td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                  {new Date(v.created_at).toLocaleDateString()}
                </td>
                <td style={{ ...tdStyle, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                  {v.visibility !== 'removed' && (
                    <select
                      style={{ ...inputStyle, padding: '2px 6px', fontSize: '0.8rem', height: 28 }}
                      value={v.visibility}
                      onChange={e => setVisibility(v.id, e.target.value)}
                    >
                      <option value="public">public</option>
                      <option value="unlisted">unlisted</option>
                      <option value="private">private</option>
                      <option value="removed">removed</option>
                    </select>
                  )}
                  <button
                    style={{ ...btnSmall, borderColor: '#dc2626', color: '#fca5a5' }}
                    onClick={() => deleteVideo(v.id)}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {videos.length === 0 && (
              <tr>
                <td colSpan={7} style={{ ...tdStyle, textAlign: 'center', color: 'var(--text-muted)' }}>
                  No videos found
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}

      <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
        <button style={btnSmall} disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>Previous</button>
        <button style={btnSmall} disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>Next</button>
      </div>
    </div>
  );
}

function videoStatusColor(s: string) {
  if (s === 'ready') return 'rgba(34,197,94,0.15)';
  if (s === 'transcoding' || s === 'queued') return 'rgba(234,179,8,0.15)';
  if (s === 'failed') return 'rgba(239,68,68,0.15)';
  return 'rgba(100,116,139,0.15)';
}

function visibilityColor(v: string) {
  if (v === 'public') return 'rgba(59,130,246,0.15)';
  if (v === 'unlisted') return 'rgba(168,85,247,0.15)';
  if (v === 'removed') return 'rgba(239,68,68,0.15)';
  return 'rgba(100,116,139,0.15)';
}

const inputStyle: React.CSSProperties = {
  padding: '6px 10px',
  background: 'var(--bg-input)',
  border: '1px solid var(--border)',
  color: 'var(--text)',
  borderRadius: 6,
  fontSize: '0.875rem',
};
const errorBanner: React.CSSProperties = {
  background: 'rgba(239,68,68,0.1)',
  color: '#fca5a5',
  padding: '10px 14px',
  borderRadius: 8,
  marginBottom: 12,
  fontSize: '0.875rem',
};
const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '10px 8px',
  borderBottom: '1px solid var(--border)',
  color: 'var(--text-muted)',
  fontSize: '0.75rem',
  textTransform: 'uppercase',
  letterSpacing: '0.06em',
};
const tdStyle: React.CSSProperties = {
  padding: '10px 8px',
  borderBottom: '1px solid var(--border)',
  fontSize: '0.875rem',
  color: 'var(--text)',
};
const badge: React.CSSProperties = {
  display: 'inline-block',
  padding: '2px 8px',
  borderRadius: 4,
  fontSize: '0.75rem',
};
const btnSmall: React.CSSProperties = {
  padding: '3px 10px',
  fontSize: '0.8rem',
  background: 'var(--bg-input)',
  border: '1px solid var(--border)',
  color: 'var(--text)',
  borderRadius: 4,
  cursor: 'pointer',
};
