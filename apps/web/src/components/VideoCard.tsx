export type VideoCardItem = {
  id: string;
  title: string;
  description?: string | null;
  owner_display_name?: string;
  duration_seconds?: number | null;
  view_count?: number;
  like_count?: number;
  published_at?: string | null;
  created_at?: string;
  thumbnail_url?: string | null;
  visibility?: string;
  media_type?: 'vod' | 'live' | string;
  status?: string;
};

type Props = {
  video: VideoCardItem;
  href: string;
  onClick?: (e: React.MouseEvent<HTMLAnchorElement>) => void;
  showVisibility?: boolean;
};

export default function VideoCard({ video, href, onClick, showVisibility = false }: Props) {
  const dateStr = video.published_at ?? video.created_at;

  return (
    <a
      href={href}
      onClick={onClick}
      className="video-card"
      aria-label={`Watch ${video.title}`}
    >
      <div className="video-card-thumb" aria-hidden="true">
        {video.thumbnail_url ? (
          <img src={video.thumbnail_url} alt="" loading="lazy" />
        ) : (
          <div className="video-card-thumb-placeholder" />
        )}
        {video.duration_seconds != null && video.duration_seconds > 0 && (
          <span className="video-card-duration">{formatDuration(video.duration_seconds)}</span>
        )}
        {video.media_type === 'live' && (
          <span className="live-badge">LIVE</span>
        )}
        {showVisibility && video.visibility && video.visibility !== 'public' && (
          <span className="video-card-visibility-badge">{video.visibility}</span>
        )}
      </div>
      <div className="video-card-body">
        <h3 className="video-card-title">{video.title}</h3>
        {video.description && (
          <p className="video-card-description">{video.description}</p>
        )}
        <div className="video-card-meta">
          {video.owner_display_name && <span>{video.owner_display_name}</span>}
          {video.view_count != null && (
            <span>{video.view_count.toLocaleString()} views</span>
          )}
          {dateStr && <span>{formatRelativeDate(dateStr)}</span>}
          {video.media_type && (
            <span className="video-card-type-badge">{video.media_type.toUpperCase()}</span>
          )}
        </div>
      </div>
    </a>
  );
}

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  return `${m}:${s.toString().padStart(2, '0')}`;
}

function formatRelativeDate(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  if (diffDays === 0) return 'today';
  if (diffDays === 1) return 'yesterday';
  if (diffDays < 7) return `${diffDays} days ago`;
  if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
  if (diffDays < 365) return `${Math.floor(diffDays / 30)} months ago`;
  return `${Math.floor(diffDays / 365)} years ago`;
}
