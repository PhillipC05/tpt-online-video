import { useCallback, useEffect, useState } from 'react';

type DashboardStats = {
  open_reports: number;
  assigned_reports: number;
  resolved_reports: number;
  dismissed_reports: number;
  pending_appeals: number;
  actions_last_24h: number;
};

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

const API_BASE = '';

async function api(path: string, options?: RequestInit) {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  const data = await res.json();
  if (!data.success) {
    throw new Error(data.error?.message || 'API error');
  }
  return data.data;
}

export default function ModerationDashboard() {
  const [tab, setTab] = useState<'dashboard' | 'reports' | 'actions' | 'audit' | 'appeals'>('dashboard');
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [reports, setReports] = useState<Report[]>([]);
  const [reportsTotal, setReportsTotal] = useState(0);
  const [actions, setActions] = useState<ModerationAction[]>([]);
  const [auditEntries, setAuditEntries] = useState<AuditEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Filters
  const [statusFilter, setStatusFilter] = useState('');
  const [targetFilter, setTargetFilter] = useState('');

  const loadReports = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);
      if (targetFilter) params.set('target_type', targetFilter);
      params.set('limit', '20');

      const data = await api(`/api/v1/admin/reports?${params.toString()}`);
      setReports(data.reports as Report[]);
      setReportsTotal(data.total as number);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load reports');
    } finally {
      setLoading(false);
    }
  }, [statusFilter, targetFilter]);

  const loadActions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (targetFilter) params.set('target_type', targetFilter);
      params.set('limit', '20');

      const data = await api(`/api/v1/admin/moderation/actions?${params.toString()}`);
      setActions(data.actions as ModerationAction[]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load actions');
    } finally {
      setLoading(false);
    }
  }, [targetFilter]);

  useEffect(() => {
    if (tab === 'dashboard') {
      loadStats();
    } else if (tab === 'reports') {
      loadReports();
    } else if (tab === 'actions') {
      loadActions();
    } else if (tab === 'audit') {
      loadAuditLog();
    }
  }, [tab, statusFilter, targetFilter, loadActions, loadReports]);

  async function loadStats() {
    setLoading(true);
    setError(null);
    try {
      const data = await api('/api/v1/admin/moderation/stats');
      setStats(data as DashboardStats);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stats');
    } finally {
      setLoading(false);
    }
  }

  async function loadAuditLog() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      params.set('limit', '20');
      const data = await api(`/api/v1/admin/audit-log?${params.toString()}`);
      setAuditEntries(data.entries as AuditEntry[]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load audit log');
    } finally {
      setLoading(false);
    }
  }

  async function handleAction(
    reportId: string,
    action: string,
    reason: string,
  ) {
    try {
      await api(`/api/v1/admin/reports/${reportId}/resolve`, {
        method: 'POST',
        body: JSON.stringify({ action_type: action, reason }),
      });
      loadReports();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Action failed');
    }
  }

  async function handleResolveAppeal(reportId: string, decision: string) {
    try {
      await api(`/api/v1/admin/reports/${reportId}/appeal`, {
        method: 'POST',
        body: JSON.stringify({ decision, notes: '' }),
      });
      loadReports();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Appeal resolution failed');
    }
  }

  async function handleAssignReport(reportId: string, assigneeId: string) {
    try {
      await api(`/api/v1/admin/reports/${reportId}/assign`, {
        method: 'POST',
        body: JSON.stringify({ assignee_id: assigneeId }),
      });
      loadReports();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Assignment failed');
    }
  }

  async function handleDismissReport(reportId: string) {
    try {
      await api(`/api/v1/admin/reports/${reportId}/dismiss`, { method: 'POST' });
      loadReports();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Dismiss failed');
    }
  }

  return (
    <div className="admin-dashboard">
      <header className="admin-header">
        <h1>Moderation Dashboard</h1>
        <nav className="admin-tabs">
          <button className={tab === 'dashboard' ? 'active' : ''} onClick={() => setTab('dashboard')}>
            Dashboard
          </button>
          <button className={tab === 'reports' ? 'active' : ''} onClick={() => setTab('reports')}>
            Reports {reportsTotal > 0 && `(${reportsTotal})`}
          </button>
          <button className={tab === 'actions' ? 'active' : ''} onClick={() => setTab('actions')}>
            Actions
          </button>
          <button className={tab === 'audit' ? 'active' : ''} onClick={() => setTab('audit')}>
            Audit Log
          </button>
        </nav>
      </header>

      {error && <div className="error-banner">{error}</div>}

      {loading && <div className="loading">Loading...</div>}

      {tab === 'dashboard' && stats && (
        <div className="stats-grid">
          <div className="stat-card open">
            <h3>{stats.open_reports}</h3>
            <p>Open Reports</p>
          </div>
          <div className="stat-card assigned">
            <h3>{stats.assigned_reports}</h3>
            <p>Assigned</p>
          </div>
          <div className="stat-card resolved">
            <h3>{stats.resolved_reports}</h3>
            <p>Resolved</p>
          </div>
          <div className="stat-card dismissed">
            <h3>{stats.dismissed_reports}</h3>
            <p>Dismissed</p>
          </div>
          <div className="stat-card appeals">
            <h3>{stats.pending_appeals}</h3>
            <p>Pending Appeals</p>
          </div>
          <div className="stat-card actions">
            <h3>{stats.actions_last_24h}</h3>
            <p>Actions (24h)</p>
          </div>
        </div>
      )}

      {tab === 'reports' && (
        <div>
          <div className="filters">
            <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)}>
              <option value="">All Status</option>
              <option value="open">Open</option>
              <option value="assigned">Assigned</option>
              <option value="resolved">Resolved</option>
              <option value="dismissed">Dismissed</option>
            </select>
            <select value={targetFilter} onChange={e => setTargetFilter(e.target.value)}>
              <option value="">All Types</option>
              <option value="video">Video</option>
              <option value="comment">Comment</option>
              <option value="user">User</option>
              <option value="live_stream">Live Stream</option>
              <option value="live_chat_message">Chat Message</option>
            </select>
          </div>

          <div className="reports-table">
            <table>
              <thead>
                <tr>
                  <th>Type</th>
                  <th>Reporter</th>
                  <th>Reason</th>
                  <th>Status</th>
                  <th>Assignee</th>
                  <th>Date</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {reports.map(r => (
                  <tr key={r.id}>
                    <td><span className="badge">{r.target_type}</span></td>
                    <td>{r.reporter_name}</td>
                    <td className="reason-cell">{r.reason.substring(0, 60)}{r.reason.length > 60 ? '...' : ''}</td>
                    <td><span className={`status-${r.status}`}>{r.status}</span></td>
                    <td>{r.assignee_name || '-'}</td>
                    <td>{new Date(r.created_at).toLocaleDateString()}</td>
                    <td className="action-buttons">
                      {r.status === 'open' && (
                        <>
                          <button onClick={() => handleAssignReport(r.id, '')} title="Assign to self">Assign</button>
                          <button onClick={() => handleDismissReport(r.id)} className="danger">Dismiss</button>
                          <select
                            defaultValue=""
                            onChange={e => {
                              if (e.target.value) {
                                handleAction(r.id, e.target.value, '');
                              }
                            }}
                          >
                            <option value="" disabled>Action...</option>
                            <option value="hide_content">Hide</option>
                            <option value="unpublish_video">Unpublish</option>
                            <option value="delete_video">Delete</option>
                            <option value="remove_comment">Remove Comment</option>
                            <option value="suspend_user">Suspend User</option>
                            <option value="ban_user">Ban User</option>
                            <option value="warn_user">Warn User</option>
                          </select>
                        </>
                      )}
                      {r.status === 'resolved' && r.appeal_status === 'pending' && (
                        <>
                          <button onClick={() => handleResolveAppeal(r.id, 'granted')}>Grant Appeal</button>
                          <button onClick={() => handleResolveAppeal(r.id, 'denied')} className="danger">Deny Appeal</button>
                        </>
                      )}
                    </td>
                  </tr>
                ))}
                {reports.length === 0 && (
                  <tr><td colSpan={7} className="empty">No reports found</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'actions' && (
        <div className="actions-table">
          <table>
            <thead>
              <tr>
                <th>Type</th>
                <th>Actor</th>
                <th>Target</th>
                <th>Reason</th>
                <th>Reversed</th>
                <th>Date</th>
              </tr>
            </thead>
            <tbody>
              {actions.map(a => (
                <tr key={a.id}>
                  <td><span className="badge">{a.action_type}</span></td>
                  <td>{a.actor_name}</td>
                  <td>{a.target_type}:{a.target_id.substring(0, 8)}</td>
                  <td className="reason-cell">{a.reason || '-'}</td>
                  <td>{a.reversed_at ? 'Yes' : 'No'}</td>
                  <td>{new Date(a.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
              {actions.length === 0 && (
                <tr><td colSpan={6} className="empty">No actions found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {tab === 'audit' && (
        <div className="audit-table">
          <table>
            <thead>
              <tr>
                <th>Actor</th>
                <th>Action</th>
                <th>Target</th>
                <th>IP</th>
                <th>Date</th>
              </tr>
            </thead>
            <tbody>
              {auditEntries.map(e => (
                <tr key={e.id}>
                  <td>{e.actor_name || 'system'}</td>
                  <td><code>{e.action}</code></td>
                  <td>{e.target_type}:{e.target_id ? e.target_id.substring(0, 8) : '-'}</td>
                  <td>{e.ip_address || '-'}</td>
                  <td>{new Date(e.created_at).toLocaleString()}</td>
                </tr>
              ))}
              {auditEntries.length === 0 && (
                <tr><td colSpan={5} className="empty">No audit entries found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <style>{`
        .admin-dashboard {
          padding: 24px;
          max-width: 1200px;
          margin: 0 auto;
        }
        .admin-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 24px;
        }
        .admin-header h1 {
          margin: 0;
          color: #e0f2fe;
        }
        .admin-tabs {
          display: flex;
          gap: 8px;
        }
        .admin-tabs button {
          padding: 8px 16px;
          border: 1px solid rgba(148, 163, 184, 0.3);
          background: transparent;
          color: #cbd5e1;
          border-radius: 6px;
          cursor: pointer;
        }
        .admin-tabs button.active {
          background: #1e40af;
          border-color: #1e40af;
          color: #fff;
        }
        .error-banner {
          background: #7f1d1d;
          color: #fecaca;
          padding: 12px;
          border-radius: 8px;
          margin-bottom: 16px;
        }
        .loading {
          text-align: center;
          color: #94a3b8;
          padding: 48px;
        }
        .stats-grid {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 16px;
        }
        .stat-card {
          padding: 24px;
          border-radius: 12px;
          text-align: center;
          border: 1px solid rgba(148, 163, 184, 0.2);
        }
        .stat-card h3 { font-size: 2rem; margin: 0 0 4px; }
        .stat-card p { margin: 0; color: #94a3b8; }
        .stat-card.open { background: rgba(234, 179, 8, 0.1); }
        .stat-card.assigned { background: rgba(59, 130, 246, 0.1); }
        .stat-card.resolved { background: rgba(34, 197, 94, 0.1); }
        .stat-card.dismissed { background: rgba(100, 116, 139, 0.1); }
        .stat-card.appeals { background: rgba(168, 85, 247, 0.1); }
        .stat-card.actions { background: rgba(236, 72, 153, 0.1); }

        .filters {
          display: flex;
          gap: 12px;
          margin-bottom: 16px;
        }
        .filters select {
          padding: 8px 12px;
          background: #1e293b;
          border: 1px solid rgba(148, 163, 184, 0.3);
          color: #e2e8f0;
          border-radius: 6px;
        }
        table {
          width: 100%;
          border-collapse: collapse;
        }
        th {
          text-align: left;
          padding: 12px 8px;
          border-bottom: 1px solid rgba(148, 163, 184, 0.2);
          color: #94a3b8;
          font-size: 0.85rem;
          text-transform: uppercase;
        }
        td {
          padding: 10px 8px;
          border-bottom: 1px solid rgba(148, 163, 184, 0.1);
          color: #e2e8f0;
          font-size: 0.9rem;
        }
        .reason-cell { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        .empty { text-align: center; color: #64748b; padding: 24px; }
        .badge {
          display: inline-block;
          padding: 2px 8px;
          background: rgba(59, 130, 246, 0.2);
          color: #93c5fd;
          border-radius: 4px;
          font-size: 0.8rem;
          text-transform: uppercase;
        }
        .status-open { color: #eab308; }
        .status-assigned { color: #60a5fa; }
        .status-resolved { color: #4ade80; }
        .status-dismissed { color: #94a3b8; }
        code {
          background: rgba(148, 163, 184, 0.15);
          padding: 2px 6px;
          border-radius: 4px;
          font-size: 0.8rem;
        }
        .action-buttons {
          display: flex;
          gap: 4px;
          align-items: center;
        }
        .action-buttons button, .action-buttons select {
          padding: 4px 8px;
          font-size: 0.8rem;
          background: #1e293b;
          border: 1px solid rgba(148, 163, 184, 0.3);
          color: #cbd5e1;
          border-radius: 4px;
          cursor: pointer;
        }
        .action-buttons button.danger { border-color: #dc2626; color: #fca5a5; }
      `}</style>
    </div>
  );
}