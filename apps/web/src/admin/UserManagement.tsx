import { useEffect, useState } from 'react';
import { apiGet, apiPatch } from '../api/client';

type AdminUser = {
  id: string;
  email: string;
  display_name: string;
  status: string;
  roles: string[];
  created_at: string;
};

export default function UserManagement() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [total, setTotal] = useState(0);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const limit = 50;

  useEffect(() => {
    load();
  }, [search, statusFilter, offset]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (search) params.set('q', search);
      if (statusFilter) params.set('status', statusFilter);
      params.set('limit', String(limit));
      params.set('offset', String(offset));
      const data = await apiGet<{ users: AdminUser[]; total: number }>(
        `/api/v1/admin/users?${params}`,
      );
      setUsers(data.users);
      setTotal(data.total);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load users');
    } finally {
      setLoading(false);
    }
  }

  async function updateStatus(userId: string, status: string) {
    setActionError(null);
    try {
      await apiPatch(`/api/v1/admin/users/${userId}`, { status });
      load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to update user');
    }
  }

  async function updateRole(userId: string, role: string) {
    setActionError(null);
    try {
      await apiPatch(`/api/v1/admin/users/${userId}`, { role });
      load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to update role');
    }
  }

  return (
    <div>
      <h2 style={heading}>User Management</h2>

      <div style={filterRow}>
        <input
          style={inputStyle}
          placeholder="Search by name or email…"
          value={search}
          onChange={e => { setSearch(e.target.value); setOffset(0); }}
        />
        <select
          style={inputStyle}
          value={statusFilter}
          onChange={e => { setStatusFilter(e.target.value); setOffset(0); }}
        >
          <option value="">All Status</option>
          <option value="active">Active</option>
          <option value="suspended">Suspended</option>
          <option value="banned">Banned</option>
        </select>
        <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem', alignSelf: 'center' }}>
          {total} total
        </span>
      </div>

      {error && <div style={errorBanner}>{error}</div>}
      {actionError && <div style={errorBanner}>{actionError}</div>}
      {loading && <div style={muted}>Loading…</div>}

      {!loading && (
        <table style={tableStyle}>
          <thead>
            <tr>
              {['Display Name', 'Email', 'Status', 'Role', 'Joined', 'Actions'].map(h => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id}>
                <td style={tdStyle}>{u.display_name}</td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', fontSize: '0.85rem' }}>{u.email}</td>
                <td style={tdStyle}>
                  <span style={{ ...badge, background: statusColor(u.status) }}>
                    {u.status}
                  </span>
                </td>
                <td style={tdStyle}>{u.roles.join(', ') || 'user'}</td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                  {new Date(u.created_at).toLocaleDateString()}
                </td>
                <td style={{ ...tdStyle, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                  {u.status === 'active' && (
                    <button style={btnSmall} onClick={() => updateStatus(u.id, 'suspended')}>
                      Suspend
                    </button>
                  )}
                  {u.status === 'suspended' && (
                    <button style={{ ...btnSmall, ...btnSuccess }} onClick={() => updateStatus(u.id, 'active')}>
                      Restore
                    </button>
                  )}
                  {u.status !== 'banned' && (
                    <button style={{ ...btnSmall, ...btnDanger }} onClick={() => updateStatus(u.id, 'banned')}>
                      Ban
                    </button>
                  )}
                  {u.status === 'banned' && (
                    <button style={{ ...btnSmall, ...btnSuccess }} onClick={() => updateStatus(u.id, 'active')}>
                      Unban
                    </button>
                  )}
                  <select
                    style={{ ...inputStyle, padding: '2px 6px', fontSize: '0.8rem', height: 28 }}
                    value={u.roles[0] ?? 'user'}
                    onChange={e => updateRole(u.id, e.target.value)}
                  >
                    <option value="user">user</option>
                    <option value="moderator">moderator</option>
                    <option value="admin">admin</option>
                  </select>
                </td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr>
                <td colSpan={6} style={{ ...tdStyle, textAlign: 'center', color: 'var(--text-muted)' }}>
                  No users found
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
        <button
          style={btnSmall}
          disabled={offset + limit >= total}
          onClick={() => setOffset(offset + limit)}
        >
          Next
        </button>
      </div>
    </div>
  );
}

function statusColor(status: string) {
  if (status === 'active') return 'rgba(34,197,94,0.15)';
  if (status === 'suspended') return 'rgba(234,179,8,0.15)';
  return 'rgba(239,68,68,0.15)';
}

const heading: React.CSSProperties = { margin: '0 0 20px', fontSize: '1.25rem' };
const filterRow: React.CSSProperties = { display: 'flex', gap: 10, marginBottom: 16, flexWrap: 'wrap' };
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
const muted: React.CSSProperties = { color: 'var(--text-muted)', padding: '24px 0' };
const tableStyle: React.CSSProperties = { width: '100%', borderCollapse: 'collapse' };
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
const btnDanger: React.CSSProperties = { borderColor: '#dc2626', color: '#fca5a5' };
const btnSuccess: React.CSSProperties = { borderColor: '#16a34a', color: '#86efac' };
