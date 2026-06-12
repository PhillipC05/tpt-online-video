import { useEffect, useState } from 'react';

type ChannelData = {
  id: string;
  display_name: string;
  bio?: string | null;
  avatar_url?: string;
  banner_url?: string;
  video_count: number;
  created_at: string;
};

type ChannelVideo = {
  id: string;
  title: string;
  status: string;
  visibility: string;
  duration_seconds?: number | null;
  view_count: number;
  created_at: string;
  thumbnail_url?: string;
};

type ChannelLiveStream = {
  id: string;
  title: string;
  status: string;
  started_at?: string | null;
  created_at: string;
};

type Props = {
  userId: string;
};

export default function ChannelPage({ userId }: Props) {
  const [channel, setChannel] = useState<ChannelData | null>(null);
  const [videos, setVideos] = useState<ChannelVideo[]>([]);
  const [liveStreams, setLiveStreams] = useState<ChannelLiveStream[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'videos' | 'live'>('videos');

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [channelRes, videosRes, liveRes] = await Promise.all([
          fetch(`/api/v1/channels/${userId}`),
          fetch(`/api/v1/channels/${userId}/videos`),
          fetch(`/api/v1/channels/${userId}/live`),
        ]);

        if (!channelRes.ok) {
          const err = await channelRes.json().catch(() => ({ error: 'channel not found' }));
          throw new Error(err.error || 'channel not found');
        }

        const channelData: ChannelData = await channelRes.json();
        const videosData: ChannelVideo[] = await videosRes.json();
        const liveData: ChannelLiveStream[] = await liveRes.json();

        if (!cancelled) {
          setChannel(channelData);
          setVideos(videosData);
          setLiveStreams(liveData);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      }
    }

    load();
    return () => { cancelled = true; };
  }, [userId]);

  if (error) {
    return (
      <main className="app-shell">
        <div className="error-state">
          <h2>Channel not found</h2>
          <p>{error}</p>
          <a href="/" className="button">Go home</a>
        </div>
      </main>
    );
  }

  if (!channel) {
    return (
      <main className="app-shell">
        <div className="loading-state">
          <div className="spinner" />
          <p>Loading channel...</p>
        </div>
      </main>
    );
  }

  const memberSince = new Date(channel.created_at).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });

  return (
    <main className="app-shell">
      {/* Banner */}
      <div
        className="channel-banner"
        style={channel.banner_url ? { backgroundImage: `url(${channel.banner_url})` } : undefined}
      >
        <div className="channel-banner-overlay" />
      </div>

      {/* Channel header */}
      <div className="channel-header">
        <div className="channel-avatar-wrapper">
          {channel.avatar_url ? (
            <img src={channel.avatar_url} alt={channel.display_name} className="channel-avatar" />
          ) : (
            <div className="channel-avatar-placeholder">
              {channel.display_name.charAt(0).toUpperCase()}
            </div>
          )}
        </div>
        <div className="channel-info">
          <h1 className="channel-name">{channel.display_name}</h1>
          {channel.bio && <p className="channel-bio">{channel.bio}</p>}
          <p className="channel-meta">
            {channel.video_count} video{channel.video_count !== 1 ? 's' : ''}
            {' · '}Member since {memberSince}
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="channel-tabs">
        <button
          className={`channel-tab ${activeTab === 'videos' ? 'channel-tab--active' : ''}`}
          onClick={() => setActiveTab('videos')}
        >
          Videos
        </button>
        <button
          className={`channel-tab ${activeTab === 'live' ? 'channel-tab--active' : ''}`}
          onClick={() => setActiveTab('live')}
        >
          Live streams
        </button>
      </div>

      {/* Video grid */}
      {activeTab === 'videos' && (
        <div className="channel-video-grid">
          {videos.length === 0 && (
            <p className="empty-state">No videos yet.</p>
          )}
          {videos.map((video) => (
            <a key={video.id} href={`/watch/${video.id}`} className="channel-video-card"
              onClick={(e) => {
                e.preventDefault();
                window.history.pushState({}, '', `/watch/${video.id}`);
                window.dispatchEvent(new PopStateEvent('popstate'));
              }}
            >
              <div className="channel-video-thumb">
                {video.thumbnail_url ? (
                  <img src={video.thumbnail_url} alt={video.title} loading="lazy" />
                ) : (
                <div className="channel-video-thumb-placeholder" />
                )}
                {video.duration_seconds && (
                  <span className="channel-video-duration">
                    {formatDuration(video.duration_seconds)}
                  </span>
                )}
                {video.visibility !== 'public' && (
                  <span className="channel-video-visibility-badge">{video.visibility}</span>
                )}
              </div>
              <div className="channel-video-info">
                <h3 className="channel-video-title">{video.title}</h3>
                <p className="channel-video-meta">
                  {video.view_count.toLocaleString()} views
                  {' · '}
                  {formatRelativeDate(video.created_at)}
                </p>
              </div>
            </a>
          ))}
        </div>
      )}

      {/* Live streams */}
      {activeTab === 'live' && (
        <div className="channel-video-grid">
          {liveStreams.length === 0 && (
            <p className="empty-state">No live streams yet.</p>
          )}
          {liveStreams.map((stream) => (
            <div key={stream.id} className="channel-video-card">
              <div className="channel-video-thumb">
                <div className="channel-live-thumb-placeholder">
                  {stream.status === 'live' && <span className="live-badge">LIVE</span>}
                </div>
              </div>
              <div className="channel-video-info">
                <h3 className="channel-video-title">{stream.title}</h3>
                <p className="channel-video-meta">
                  Status: {stream.status}
                  {stream.started_at && (
                    <> · Started {formatRelativeDate(stream.started_at)}</>
                  )}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </main>
  );
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return '0:00';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  }
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