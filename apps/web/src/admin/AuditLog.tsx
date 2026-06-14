import { useEffect, useState } from 'react';
import { apiGet } from '../api/client';

type AuditEntry = {
  id: string;
  actor_id: string | null;
  action: string;
  target_type: string | null;
  target_id: string | null;
  metadata: Record<string, unknown>;
  ip_address: string | null;
  created_at: string;
  actor_name: string;
};

export default function AuditLog() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(true);

  const limit = 50;

  useEffect(() => { load(); }, [offset]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
      const data = await apiGet<{ entries: AuditEntry[] }>(
        `/api/v1/admin/audit-log?${params}`,
      );
      setEntries(data.entries);
      setHasMore(data.entries.length === limit);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load audit log');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 20px', fontSize: '1.25rem' }}>Audit Log</h2>

      {error && <div style={errorBanner}>{error}</div>}
      {loading && <div style={{ color: 'var(--text-muted)', padding: '24px 0' }}>Loading…</div>}

      {!loading && (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Actor', 'Action', 'Target', 'IP Address', 'Timestamp'].map(h => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {entries.map(e => (
              <tr key={e.id}>
                <td style={{ ...tdStyle, color: 'var(--text-muted)' }}>{e.actor_name || 'system'}</td>
                <td style={tdStyle}>
                  <code style={codeStyle}>{e.action}</code>
                </td>
                <td style={{ ...tdStyle, fontFamily: 'monospace', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  {e.target_type
                    ? `${e.target_type}:${e.target_id ? e.target_id.substring(0, 8) : '-'}`
                    : '-'}
                </td>
                <td style={{ ...tdStyle, fontFamily: 'monospace', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  {e.ip_address || '-'}
                </td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', fontSize: '0.8rem', whiteSpace: 'nowrap' }}>
                  {new Date(e.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
            {entries.length === 0 && (
              <tr>
                <td colSpan={5} style={{ ...tdStyle, textAlign: 'center', color: 'var(--text-muted)' }}>
                  No audit entries found
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}

      <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
        <button style={btnSmall} disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>
          Previous
        </button>
        <button style={btnSmall} disabled={!hasMore} onClick={() => setOffset(offset + limit)}>
          Next
        </button>
      </div>
    </div>
  );
}

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
const codeStyle: React.CSSProperties = {
  background: 'rgba(148,163,184,0.12)', padding: '2px 6px', borderRadius: 4,
  fontFamily: 'monospace', fontSize: '0.8rem',
};
const btnSmall: React.CSSProperties = {
  padding: '3px 10px', fontSize: '0.8rem', background: 'var(--bg-input)',
  border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4, cursor: 'pointer',
};
