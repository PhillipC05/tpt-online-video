import { useEffect, useState } from 'react';
import { apiGet, apiPatch } from '../api/client';

type AdminComment = {
  id: string;
  body: string;
  author_name: string;
  video_title: string;
  video_id: string;
  status: string;
  created_at: string;
};

export default function CommentManagement() {
  const [comments, setComments] = useState<AdminComment[]>([]);
  const [total, setTotal] = useState(0);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const limit = 50;

  useEffect(() => { load(); }, [search, statusFilter, offset]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (search) params.set('q', search);
      if (statusFilter) params.set('status', statusFilter);
      params.set('limit', String(limit));
      params.set('offset', String(offset));
      const data = await apiGet<{ comments: AdminComment[]; total: number }>(
        `/api/v1/admin/comments?${params}`,
      );
      setComments(data.comments);
      setTotal(data.total);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load comments');
    } finally {
      setLoading(false);
    }
  }

  async function updateStatus(commentId: string, status: string) {
    setActionError(null);
    try {
      await apiPatch(`/api/v1/admin/comments/${commentId}`, { status });
      load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to update comment');
    }
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 20px', fontSize: '1.25rem' }}>Comment Management</h2>

      <div style={{ display: 'flex', gap: 10, marginBottom: 16, flexWrap: 'wrap' }}>
        <input
          style={inputStyle}
          placeholder="Search comment text…"
          value={search}
          onChange={e => { setSearch(e.target.value); setOffset(0); }}
        />
        <select style={inputStyle} value={statusFilter} onChange={e => { setStatusFilter(e.target.value); setOffset(0); }}>
          <option value="">All Status</option>
          <option value="visible">Visible</option>
          <option value="hidden">Hidden</option>
          <option value="deleted">Deleted</option>
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
              {['Author', 'Comment', 'Video', 'Status', 'Date', 'Actions'].map(h => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {comments.map(c => (
              <tr key={c.id}>
                <td style={{ ...tdStyle, color: 'var(--text-muted)' }}>{c.author_name}</td>
                <td style={{ ...tdStyle, maxWidth: 280, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {c.body}
                </td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {c.video_title}
                </td>
                <td style={tdStyle}>
                  <span style={{ ...badge, background: commentStatusColor(c.status) }}>{c.status}</span>
                </td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                  {new Date(c.created_at).toLocaleDateString()}
                </td>
                <td style={{ ...tdStyle, display: 'flex', gap: 6 }}>
                  {c.status === 'visible' && (
                    <button style={btnSmall} onClick={() => updateStatus(c.id, 'hidden')}>Hide</button>
                  )}
                  {c.status === 'hidden' && (
                    <button style={{ ...btnSmall, borderColor: '#16a34a', color: '#86efac' }}
                      onClick={() => updateStatus(c.id, 'visible')}>
                      Restore
                    </button>
                  )}
                  {c.status !== 'deleted' && (
                    <button
                      style={{ ...btnSmall, borderColor: '#dc2626', color: '#fca5a5' }}
                      onClick={() => updateStatus(c.id, 'deleted')}
                    >
                      Delete
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {comments.length === 0 && (
              <tr>
                <td colSpan={6} style={{ ...tdStyle, textAlign: 'center', color: 'var(--text-muted)' }}>
                  No comments found
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

function commentStatusColor(s: string) {
  if (s === 'visible') return 'rgba(34,197,94,0.15)';
  if (s === 'hidden') return 'rgba(234,179,8,0.15)';
  return 'rgba(239,68,68,0.15)';
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
