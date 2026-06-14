import { useEffect, useState } from 'react';
import { apiGet, apiPost } from '../api/client';

type ModerationAction = {
  id: string;
  actor_id: string;
  report_id: string | null;
  target_type: string;
  target_id: string;
  action_type: string;
  reason: string | null;
  admin_notes: string | null;
  reversed_by: string | null;
  reversed_at: string | null;
  created_at: string;
  actor_name: string;
};

export default function ModerationHistory() {
  const [actions, setActions] = useState<ModerationAction[]>([]);
  const [targetFilter, setTargetFilter] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => { load(); }, [targetFilter]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({ limit: '50' });
      if (targetFilter) params.set('target_type', targetFilter);
      const data = await apiGet<{ actions: ModerationAction[] }>(
        `/api/v1/admin/moderation/actions?${params}`,
      );
      setActions(data.actions);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load actions');
    } finally {
      setLoading(false);
    }
  }

  async function reverse(actionId: string) {
    try {
      await apiPost(`/api/v1/admin/moderation/actions/${actionId}/reverse`, {});
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to reverse action');
    }
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 20px', fontSize: '1.25rem' }}>Moderation Action History</h2>

      <div style={{ display: 'flex', gap: 10, marginBottom: 16 }}>
        <select
          style={inputStyle}
          value={targetFilter}
          onChange={e => setTargetFilter(e.target.value)}
        >
          <option value="">All Types</option>
          <option value="video">Video</option>
          <option value="comment">Comment</option>
          <option value="user">User</option>
          <option value="live_stream">Live Stream</option>
        </select>
      </div>

      {error && <div style={errorBanner}>{error}</div>}
      {loading && <div style={{ color: 'var(--text-muted)', padding: '24px 0' }}>Loading…</div>}

      {!loading && (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Action', 'Actor', 'Target', 'Reason', 'Reversed', 'Date', ''].map((h, i) => (
                <th key={i} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {actions.map(a => (
              <tr key={a.id}>
                <td style={tdStyle}><span style={badge}>{a.action_type}</span></td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)' }}>{a.actor_name}</td>
                <td style={{ ...tdStyle, fontFamily: 'monospace', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  {a.target_type}:{a.target_id.substring(0, 8)}
                </td>
                <td style={{ ...tdStyle, maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--text-muted)' }}>
                  {a.reason || '-'}
                </td>
                <td style={tdStyle}>
                  {a.reversed_at ? (
                    <span style={{ color: '#4ade80', fontSize: '0.8rem' }}>Reversed</span>
                  ) : (
                    <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>—</span>
                  )}
                </td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                  {new Date(a.created_at).toLocaleString()}
                </td>
                <td style={tdStyle}>
                  {!a.reversed_at && (
                    <button style={btnSmall} onClick={() => reverse(a.id)}>Reverse</button>
                  )}
                </td>
              </tr>
            ))}
            {actions.length === 0 && (
              <tr>
                <td colSpan={7} style={{ ...tdStyle, textAlign: 'center', color: 'var(--text-muted)' }}>
                  No actions found
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  );
}

const inputStyle: React.CSSProperties = {
  padding: '6px 10px', background: 'var(--bg-input)', border: '1px solid var(--border)',
  color: 'var(--text)', borderRadius: 6, fontSize: '0.875rem',
};
const errorBanner: React.CSSProperties = {
  background: 'rgba(239,68,68,0.1)', color: '#fca5a5', padding: '10px 14px',
  borderRadius: 8, marginBottom: 12, fontSize: '0.875rem',
};
const thStyle: React.CSSProperties = {
  textAlign: 'left', padding: '10px 8px', borderBottom: '1px solid var(--border)',
  color: 'var(--text-muted)', fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.06em',
};
const tdStyle: React.CSSProperties = {
  padding: '10px 8px', borderBottom: '1px solid var(--border)', fontSize: '0.875rem', color: 'var(--text)',
};
const badge: React.CSSProperties = {
  display: 'inline-block', padding: '2px 8px', background: 'rgba(168,85,247,0.15)',
  color: '#c4b5fd', borderRadius: 4, fontSize: '0.75rem',
};
const btnSmall: React.CSSProperties = {
  padding: '3px 8px', fontSize: '0.75rem', background: 'var(--bg-input)',
  border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4, cursor: 'pointer',
};
