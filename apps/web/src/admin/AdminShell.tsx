import { useState } from 'react';
import AdminHome from './AdminHome';
import UserManagement from './UserManagement';
import VideoManagement from './VideoManagement';
import CommentManagement from './CommentManagement';
import ReportQueue from './ReportQueue';
import ModerationHistory from './ModerationHistory';
import AuditLog from './AuditLog';
import SystemStatus from './SystemStatus';
import AdminSettings from './AdminSettings';

type AdminSection =
  | 'home'
  | 'users'
  | 'videos'
  | 'comments'
  | 'reports'
  | 'history'
  | 'audit'
  | 'status'
  | 'settings';

const NAV_ITEMS: { key: AdminSection; label: string; icon: string }[] = [
  { key: 'home',     label: 'Overview',          icon: '⊞' },
  { key: 'users',    label: 'Users',             icon: '👤' },
  { key: 'videos',   label: 'Videos',            icon: '▶' },
  { key: 'comments', label: 'Comments',          icon: '💬' },
  { key: 'reports',  label: 'Report Queue',      icon: '⚑' },
  { key: 'history',  label: 'Mod Actions',       icon: '⚡' },
  { key: 'audit',    label: 'Audit Log',         icon: '📋' },
  { key: 'status',   label: 'System Status',     icon: '◉' },
  { key: 'settings', label: 'Settings',          icon: '⚙' },
];

export default function AdminShell() {
  const [section, setSection] = useState<AdminSection>('home');

  return (
    <div style={styles.shell}>
      <aside style={styles.sidebar}>
        <div style={styles.sidebarHeader}>
          <span style={styles.sidebarTitle}>Admin</span>
        </div>
        <nav>
          {NAV_ITEMS.map(({ key, label, icon }) => (
            <button
              key={key}
              style={{
                ...styles.navItem,
                ...(section === key ? styles.navItemActive : {}),
              }}
              onClick={() => setSection(key)}
            >
              <span style={styles.navIcon}>{icon}</span>
              {label}
            </button>
          ))}
        </nav>
      </aside>

      <main style={styles.content}>
        {section === 'home'     && <AdminHome     onNavigate={setSection} />}
        {section === 'users'    && <UserManagement />}
        {section === 'videos'   && <VideoManagement />}
        {section === 'comments' && <CommentManagement />}
        {section === 'reports'  && <ReportQueue />}
        {section === 'history'  && <ModerationHistory />}
        {section === 'audit'    && <AuditLog />}
        {section === 'status'   && <SystemStatus />}
        {section === 'settings' && <AdminSettings />}
      </main>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  shell: {
    display: 'flex',
    minHeight: 'calc(100vh - 56px)',
    background: 'var(--bg)',
  },
  sidebar: {
    width: 220,
    flexShrink: 0,
    background: 'var(--bg-surface)',
    borderRight: '1px solid var(--border)',
    display: 'flex',
    flexDirection: 'column',
  },
  sidebarHeader: {
    padding: '20px 16px 12px',
    borderBottom: '1px solid var(--border)',
  },
  sidebarTitle: {
    fontSize: '0.75rem',
    fontWeight: 700,
    letterSpacing: '0.1em',
    textTransform: 'uppercase',
    color: 'var(--text-muted)',
  },
  navItem: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    width: '100%',
    padding: '10px 16px',
    background: 'transparent',
    border: 'none',
    color: 'var(--text-subtle)',
    fontSize: '0.875rem',
    cursor: 'pointer',
    textAlign: 'left',
    borderRadius: 0,
    transition: 'background 0.15s, color 0.15s',
  },
  navItemActive: {
    background: 'rgba(96, 165, 250, 0.12)',
    color: 'var(--accent)',
    fontWeight: 600,
  },
  navIcon: {
    fontSize: '1rem',
    width: 20,
    textAlign: 'center',
    flexShrink: 0,
  },
  content: {
    flex: 1,
    padding: '28px 32px',
    minWidth: 0,
    overflowY: 'auto',
  },
};
