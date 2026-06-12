type Props = {
  title: string;
  description?: string;
  action?: {
    label: string;
    href?: string;
    onClick?: () => void;
  };
  icon?: 'video' | 'search' | 'live' | 'generic';
};

const icons: Record<NonNullable<Props['icon']>, string> = {
  video: '🎬',
  search: '🔍',
  live: '📡',
  generic: '📭',
};

export default function EmptyState({ title, description, action, icon = 'generic' }: Props) {
  return (
    <div className="empty-state" role="status" aria-label={title}>
      <span className="empty-state-icon" aria-hidden="true">{icons[icon]}</span>
      <h3 className="empty-state-title">{title}</h3>
      {description && <p className="empty-state-description">{description}</p>}
      {action && (
        action.href ? (
          <a href={action.href} className="button">{action.label}</a>
        ) : (
          <button type="button" className="button" onClick={action.onClick}>
            {action.label}
          </button>
        )
      )}
    </div>
  );
}
