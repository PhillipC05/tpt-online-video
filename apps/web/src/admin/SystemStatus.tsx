import { useEffect, useState } from 'react';
import { apiGet } from '../api/client';

type QueueStatus = {
  stream_length: number;
  undelivered_count: number;
  pending_count: number;
  consumer_count: number;
  dlq_length: number;
  oldest_pending_age?: string;
};

type LiveStatus = {
  active_streams: number;
  idle_streams: number;
};

type Status = {
  api: string;
  postgres: string;
  redis: string;
  storage: string;
  search: string;
  queue: QueueStatus | null;
  live: LiveStatus | null;
};

function StatusRow({ label, value }: { label: string; value: string }) {
  const ok = value === 'ok';
  return (
    <div style={rowStyle}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <span style={{
          width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
          background: ok ? '#4ade80' : '#f87171',
          boxShadow: ok ? '0 0 6px #4ade8066' : '0 0 6px #f8717166',
        }} />
        <span style={{ fontWeight: 600, fontSize: '0.9rem' }}>{label}</span>
      </div>
      <span style={{ color: ok ? '#4ade80' : '#f87171', fontSize: '0.85rem', fontFamily: 'monospace' }}>
        {value}
      </span>
    </div>
  );
}

function MetricRow({ label, value, warn }: { label: string; value: string | number; warn?: boolean }) {
  return (
    <div style={rowStyle}>
      <span style={{ fontSize: '0.9rem', color: 'var(--text-subtle)' }}>{label}</span>
      <span style={{
        fontSize: '0.9rem',
        fontFamily: 'monospace',
        color: warn ? '#fbbf24' : 'var(--text)',
        fontWeight: warn ? 600 : 400,
      }}>
        {value}
      </span>
    </div>
  );
}

export default function SystemStatus() {
  const [status, setStatus] = useState<Status | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastRefresh, setLastRefresh] = useState(new Date());

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const data = await apiGet<Status>('/api/v1/admin/system/status');
      setStatus(data);
      setLastRefresh(new Date());
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load system status');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 24 }}>
        <h2 style={{ margin: 0, fontSize: '1.25rem' }}>System Status</h2>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>
            Last updated: {lastRefresh.toLocaleTimeString()}
          </span>
          <button style={btnStyle} onClick={load} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </div>

      {error && (
        <div style={errorBanner}>{error}</div>
      )}

      {status && (
        <>
          <section style={cardStyle}>
            <h3 style={cardHeading}>Service Health</h3>
            <StatusRow label="API" value={status.api} />
            <StatusRow label="PostgreSQL" value={status.postgres} />
            <StatusRow label="Redis" value={status.redis} />
            <StatusRow label="Storage" value={status.storage} />
            <StatusRow label="Search" value={status.search} />
          </section>

          {status.queue && (
            <section style={cardStyle}>
              <h3 style={cardHeading}>Transcoding Queue</h3>
              <MetricRow label="Queue Backlog (undelivered)" value={status.queue.undelivered_count} warn={status.queue.undelivered_count > 10} />
              <MetricRow label="In-Progress (pending ack)" value={status.queue.pending_count} />
              <MetricRow label="Active Workers (consumers)" value={status.queue.consumer_count} warn={status.queue.consumer_count === 0} />
              <MetricRow label="Total Stream Length" value={status.queue.stream_length} />
              <MetricRow label="Dead-Letter Queue" value={status.queue.dlq_length} warn={status.queue.dlq_length > 0} />
              {status.queue.oldest_pending_age && (
                <MetricRow label="Oldest Pending Job Age" value={status.queue.oldest_pending_age} warn />
              )}
            </section>
          )}

          {status.live && (
            <section style={cardStyle}>
              <h3 style={cardHeading}>Live Streaming</h3>
              <MetricRow label="Active Live Streams" value={status.live.active_streams} />
              <MetricRow label="Idle Streams (configured)" value={status.live.idle_streams} />
            </section>
          )}
        </>
      )}
    </div>
  );
}

const cardStyle: React.CSSProperties = {
  background: 'var(--bg-surface)',
  border: '1px solid var(--border)',
  borderRadius: 12,
  padding: '20px 24px',
  marginBottom: 16,
};

const cardHeading: React.CSSProperties = {
  margin: '0 0 16px',
  fontSize: '0.8rem',
  textTransform: 'uppercase',
  letterSpacing: '0.08em',
  color: 'var(--text-muted)',
  fontWeight: 600,
};

const rowStyle: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  padding: '8px 0',
  borderBottom: '1px solid var(--border)',
};

const errorBanner: React.CSSProperties = {
  background: 'rgba(239,68,68,0.1)',
  color: '#fca5a5',
  padding: '10px 14px',
  borderRadius: 8,
  marginBottom: 12,
  fontSize: '0.875rem',
};

const btnStyle: React.CSSProperties = {
  padding: '6px 14px',
  background: 'var(--bg-input)',
  border: '1px solid var(--border)',
  color: 'var(--text)',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: '0.875rem',
};
