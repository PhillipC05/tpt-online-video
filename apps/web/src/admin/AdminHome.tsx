import { useEffect, useState } from 'react';
import { apiGet } from '../api/client';

type AdminSection = 'home' | 'users' | 'videos' | 'comments' | 'reports' | 'history' | 'audit' | 'status' | 'settings';

type ModerationStats = {
  open_reports: number;
  assigned_reports: number;
  resolved_reports: number;
  dismissed_reports: number;
  pending_appeals: number;
  actions_last_24h: number;
};

type SystemStatus = {
  api: string;
  postgres: string;
  redis: string;
  storage: string;
  queue?: {
    undelivered_count: number;
    dlq_length: number;
  };
  live?: {
    active_streams: number;
  };
};

type StatCardProps = {
  label: string;
  value: number | string;
  color?: string;
  onClick?: () => void;
};

function StatCard({ label, value, color, onClick }: StatCardProps) {
  return (
    <div
      style={{
        padding: '20px 24px',
        borderRadius: 12,
        border: '1px solid var(--border)',
        background: color ? `rgba(${color}, 0.08)` : 'var(--bg-surface)',
        cursor: onClick ? 'pointer' : 'default',
      }}
      onClick={onClick}
    >
      <div style={{ fontSize: '1.75rem', fontWeight: 700, lineHeight: 1 }}>{value}</div>
      <div style={{ marginTop: 6, fontSize: '0.85rem', color: 'var(--text-muted)' }}>{label}</div>
    </div>
  );
}

function StatusDot({ ok }: { ok: boolean }) {
  return (
    <span style={{
      display: 'inline-block',
      width: 8,
      height: 8,
      borderRadius: '50%',
      background: ok ? '#4ade80' : '#f87171',
      marginRight: 8,
    }} />
  );
}

type Props = { onNavigate: (section: AdminSection) => void };

export default function AdminHome({ onNavigate }: Props) {
  const [modStats, setModStats] = useState<ModerationStats | null>(null);
  const [sysStatus, setSysStatus] = useState<SystemStatus | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      apiGet<ModerationStats>('/api/v1/admin/moderation/stats'),
      apiGet<SystemStatus>('/api/v1/admin/system/status'),
    ])
      .then(([ms, ss]) => {
        setModStats(ms);
        setSysStatus(ss);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <div style={{ color: 'var(--text-muted)', padding: 32 }}>Loading…</div>;
  }

  const allHealthy = sysStatus
    ? [sysStatus.api, sysStatus.postgres, sysStatus.redis, sysStatus.storage].every(s => s === 'ok')
    : false;

  return (
    <div>
      <h2 style={{ margin: '0 0 24px', fontSize: '1.25rem' }}>Admin Overview</h2>

      {modStats && (
        <>
          <h3 style={sectionHeading}>Moderation</h3>
          <div style={grid}>
            <StatCard label="Open Reports" value={modStats.open_reports} color="234,179,8" onClick={() => onNavigate('reports')} />
            <StatCard label="Assigned" value={modStats.assigned_reports} color="59,130,246" onClick={() => onNavigate('reports')} />
            <StatCard label="Resolved" value={modStats.resolved_reports} color="34,197,94" />
            <StatCard label="Dismissed" value={modStats.dismissed_reports} color="100,116,139" />
            <StatCard label="Pending Appeals" value={modStats.pending_appeals} color="168,85,247" onClick={() => onNavigate('reports')} />
            <StatCard label="Actions (24h)" value={modStats.actions_last_24h} color="236,72,153" onClick={() => onNavigate('history')} />
          </div>
        </>
      )}

      {sysStatus && (
        <>
          <h3 style={{ ...sectionHeading, marginTop: 32 }}>
            System Health
            <span style={{
              marginLeft: 10,
              fontSize: '0.8rem',
              color: allHealthy ? '#4ade80' : '#f87171',
              fontWeight: 500,
            }}>
              {allHealthy ? '● All systems operational' : '● Degraded'}
            </span>
          </h3>
          <div style={statusGrid}>
            {([
              ['API', sysStatus.api],
              ['Postgres', sysStatus.postgres],
              ['Redis', sysStatus.redis],
              ['Storage', sysStatus.storage],
            ] as [string, string][]).map(([name, val]) => (
              <div key={name} style={statusItem}>
                <StatusDot ok={val === 'ok'} />
                <span style={{ fontWeight: 500, marginRight: 8 }}>{name}</span>
                <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>{val}</span>
              </div>
            ))}
          </div>

          <div style={{ ...grid, marginTop: 16 }}>
            {sysStatus.queue && (
              <StatCard
                label="Queue Backlog"
                value={sysStatus.queue.undelivered_count}
                color={sysStatus.queue.undelivered_count > 10 ? '234,179,8' : '34,197,94'}
                onClick={() => onNavigate('status')}
              />
            )}
            {sysStatus.queue && (
              <StatCard
                label="Dead-Letter Queue"
                value={sysStatus.queue.dlq_length}
                color={sysStatus.queue.dlq_length > 0 ? '239,68,68' : '34,197,94'}
                onClick={() => onNavigate('status')}
              />
            )}
            {sysStatus.live && (
              <StatCard
                label="Live Streams"
                value={sysStatus.live.active_streams}
                color="59,130,246"
              />
            )}
          </div>
        </>
      )}
    </div>
  );
}

const sectionHeading: React.CSSProperties = {
  margin: '0 0 14px',
  fontSize: '0.8rem',
  textTransform: 'uppercase',
  letterSpacing: '0.08em',
  color: 'var(--text-muted)',
  fontWeight: 600,
};

const grid: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
  gap: 14,
};

const statusGrid: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
  gap: 10,
};

const statusItem: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  padding: '10px 16px',
  borderRadius: 8,
  border: '1px solid var(--border)',
  background: 'var(--bg-surface)',
  fontSize: '0.875rem',
};
