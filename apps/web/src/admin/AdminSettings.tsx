import { useEffect, useState } from 'react';
import { apiGet } from '../api/client';

type StorageSettings = { provider: string; note: string };
type SearchSettings = { provider: string; note: string };
type ModerationSettings = { auto_mod_enabled: boolean; note: string };
type LiveSettings = { dvr_enabled: boolean; dvr_window_seconds: number; note: string };

type Settings = {
  storage: StorageSettings;
  search: SearchSettings;
  moderation: ModerationSettings;
  live: LiveSettings;
};

function SettingItem({ label, value, note }: { label: string; value: string | number | boolean; note?: string }) {
  return (
    <div style={itemStyle}>
      <div>
        <div style={{ fontWeight: 500, fontSize: '0.9rem', marginBottom: 2 }}>{label}</div>
        {note && <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', maxWidth: 480 }}>{note}</div>}
      </div>
      <div style={{ fontFamily: 'monospace', fontSize: '0.9rem', color: 'var(--accent)', flexShrink: 0 }}>
        {String(value)}
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section style={cardStyle}>
      <h3 style={cardHeading}>{title}</h3>
      {children}
    </section>
  );
}

export default function AdminSettings() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiGet<Settings>('/api/v1/admin/settings')
      .then(setSettings)
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load settings'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div style={{ color: 'var(--text-muted)', padding: 32 }}>Loading…</div>;

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h2 style={{ margin: '0 0 6px', fontSize: '1.25rem' }}>Admin Settings</h2>
        <p style={{ margin: 0, color: 'var(--text-muted)', fontSize: '0.875rem' }}>
          Current runtime configuration. Settings are managed via environment variables and require a service restart to change.
        </p>
      </div>

      {error && (
        <div style={errorBanner}>{error}</div>
      )}

      {settings && (
        <>
          <Section title="Storage Provider">
            <SettingItem
              label="Provider"
              value={settings.storage.provider}
              note={settings.storage.note}
            />
          </Section>

          <Section title="Search Provider">
            <SettingItem
              label="Provider"
              value={settings.search.provider}
              note={settings.search.note}
            />
          </Section>

          <Section title="Moderation">
            <SettingItem
              label="Auto-Moderation Enabled"
              value={settings.moderation.auto_mod_enabled}
              note={settings.moderation.note}
            />
          </Section>

          <Section title="Live Streaming">
            <SettingItem
              label="DVR Enabled"
              value={settings.live.dvr_enabled}
            />
            <SettingItem
              label="DVR Window (seconds)"
              value={settings.live.dvr_window_seconds}
              note={settings.live.note}
            />
          </Section>

          <section style={{ ...cardStyle, background: 'rgba(59,130,246,0.05)' }}>
            <h3 style={cardHeading}>Configuration Reference</h3>
            <div style={{ fontSize: '0.85rem', color: 'var(--text-subtle)', lineHeight: 1.7 }}>
              <p style={{ margin: '0 0 8px' }}>
                <strong>Storage:</strong> Set <code style={codeStyle}>STORAGE_PROVIDER</code> to{' '}
                <code style={codeStyle}>local</code>, <code style={codeStyle}>s3</code>, or{' '}
                <code style={codeStyle}>wasabi</code>. Configure bucket credentials with{' '}
                <code style={codeStyle}>STORAGE_BUCKET</code>, <code style={codeStyle}>STORAGE_REGION</code>, etc.
              </p>
              <p style={{ margin: '0 0 8px' }}>
                <strong>Search:</strong> Set <code style={codeStyle}>SEARCH_PROVIDER</code> to{' '}
                <code style={codeStyle}>postgres</code> or <code style={codeStyle}>meilisearch</code>. For Meilisearch, also set{' '}
                <code style={codeStyle}>MEILISEARCH_URL</code> and <code style={codeStyle}>MEILISEARCH_KEY</code>.
              </p>
              <p style={{ margin: '0 0 8px' }}>
                <strong>Live:</strong> Configure via <code style={codeStyle}>MEDIAMTX_HLS_BASE</code>,{' '}
                <code style={codeStyle}>MEDIAMTX_WEBRTC_BASE</code>, and <code style={codeStyle}>RTMP_BASE</code>.
              </p>
              <p style={{ margin: 0 }}>
                <strong>Security:</strong> Always set <code style={codeStyle}>JWT_SECRET</code> and{' '}
                <code style={codeStyle}>LIVE_HOOK_SECRET</code> to strong random values in production.
              </p>
            </div>
          </section>
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

const itemStyle: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  gap: 16,
  padding: '10px 0',
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

const codeStyle: React.CSSProperties = {
  background: 'rgba(148,163,184,0.12)',
  padding: '1px 5px',
  borderRadius: 3,
  fontFamily: 'monospace',
  fontSize: '0.8rem',
};
