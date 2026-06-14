import { useEffect, useState } from 'react';
import { apiGet, apiPost } from '../api/client';

type Report = {
  id: string;
  reporter_id: string;
  assignee_id: string | null;
  target_type: string;
  target_id: string;
  reason: string;
  status: string;
  priority: number;
  created_at: string;
  reporter_name: string;
  assignee_name: string;
  appeal_status?: string;
};

export default function ReportQueue() {
  const [reports, setReports] = useState<Report[]>([]);
  const [total, setTotal] = useState(0);
  const [statusFilter, setStatusFilter] = useState('open');
  const [targetFilter, setTargetFilter] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => { load(); }, [statusFilter, targetFilter]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({ limit: '50' });
      if (statusFilter) params.set('status', statusFilter);
      if (targetFilter) params.set('target_type', targetFilter);
      const data = await apiGet<{ reports: Report[]; total: number }>(
        `/api/v1/admin/reports?${params}`,
      );
      setReports(data.reports);
      setTotal(data.total);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load reports');
    } finally {
      setLoading(false);
    }
  }

  async function handleAction(reportId: string, actionType: string) {
    try {
      await apiPost(`/api/v1/admin/reports/${reportId}/resolve`, { action_type: actionType, reason: '' });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Action failed');
    }
  }

  async function handleDismiss(reportId: string) {
    try {
      await apiPost(`/api/v1/admin/reports/${reportId}/dismiss`, {});
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Dismiss failed');
    }
  }

  async function handleAppeal(reportId: string, decision: string) {
    try {
      await apiPost(`/api/v1/admin/reports/${reportId}/appeal`, { decision, notes: '' });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Appeal resolution failed');
    }
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 20px', fontSize: '1.25rem' }}>
        Report Queue
        {total > 0 && <span style={{ marginLeft: 10, color: 'var(--text-muted)', fontWeight: 400, fontSize: '1rem' }}>({total})</span>}
      </h2>

      <div style={{ display: 'flex', gap: 10, marginBottom: 16 }}>
        <select style={inputStyle} value={statusFilter} onChange={e => setStatusFilter(e.target.value)}>
          <option value="">All Status</option>
          <option value="open">Open</option>
          <option value="assigned">Assigned</option>
          <option value="resolved">Resolved</option>
          <option value="dismissed">Dismissed</option>
        </select>
        <select style={inputStyle} value={targetFilter} onChange={e => setTargetFilter(e.target.value)}>
          <option value="">All Types</option>
          <option value="video">Video</option>
          <option value="comment">Comment</option>
          <option value="user">User</option>
          <option value="live_stream">Live Stream</option>
          <option value="live_chat_message">Chat Message</option>
        </select>
      </div>

      {error && <div style={errorBanner}>{error}</div>}
      {loading && <div style={{ color: 'var(--text-muted)', padding: '24px 0' }}>Loading…</div>}

      {!loading && (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Type', 'Reporter', 'Reason', 'Status', 'Assignee', 'Date', 'Actions'].map(h => (
                <th key={h} style={thStyle}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {reports.map(r => (
              <tr key={r.id}>
                <td style={tdStyle}><span style={badge}>{r.target_type}</span></td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)' }}>{r.reporter_name}</td>
                <td style={{ ...tdStyle, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {r.reason}
                </td>
                <td style={tdStyle}>
                  <span style={{ color: statusColor(r.status) }}>{r.status}</span>
                </td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)' }}>{r.assignee_name || '-'}</td>
                <td style={{ ...tdStyle, color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                  {new Date(r.created_at).toLocaleDateString()}
                </td>
                <td style={{ ...tdStyle, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  {r.status === 'open' && (
                    <>
                      <button style={btnSmall} onClick={() => handleDismiss(r.id)}>Dismiss</button>
                      <select
                        style={{ ...inputStyle, padding: '2px 6px', fontSize: '0.75rem', height: 26 }}
                        defaultValue=""
                        onChange={e => { if (e.target.value) { handleAction(r.id, e.target.value); e.target.value = ''; } }}
                      >
                        <option value="" disabled>Action…</option>
                        <option value="hide_content">Hide</option>
                        <option value="unpublish_video">Unpublish</option>
                        <option value="delete_video">Delete Video</option>
                        <option value="remove_comment">Remove Comment</option>
                        <option value="suspend_user">Suspend User</option>
                        <option value="ban_user">Ban User</option>
                        <option value="warn_user">Warn User</option>
                      </select>
                    </>
                  )}
                  {r.status === 'resolved' && r.appeal_status === 'pending' && (
                    <>
                      <button style={{ ...btnSmall, borderColor: '#16a34a', color: '#86efac' }} onClick={() => handleAppeal(r.id, 'granted')}>Grant</button>
                      <button style={{ ...btnSmall, borderColor: '#dc2626', color: '#fca5a5' }} onClick={() => handleAppeal(r.id, 'denied')}>Deny</button>
                    </>
                  )}
                </td>
              </tr>
            ))}
            {reports.length === 0 && (
              <tr>
                <td colSpan={7} style={{ ...tdStyle, textAlign: 'center', color: 'var(--text-muted)' }}>No reports found</td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  );
}

function statusColor(s: string) {
  if (s === 'open') return '#eab308';
  if (s === 'assigned') return '#60a5fa';
  if (s === 'resolved') return '#4ade80';
  return 'var(--text-muted)';
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
  display: 'inline-block', padding: '2px 8px', background: 'rgba(59,130,246,0.15)',
  color: '#93c5fd', borderRadius: 4, fontSize: '0.75rem', textTransform: 'uppercase',
};
const btnSmall: React.CSSProperties = {
  padding: '3px 8px', fontSize: '0.75rem', background: 'var(--bg-input)',
  border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4, cursor: 'pointer',
};
