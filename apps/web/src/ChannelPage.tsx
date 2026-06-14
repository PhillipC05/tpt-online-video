import { useEffect, useState } from 'react';
import VideoGrid from './components/VideoGrid';
import EmptyState from './components/EmptyState';
import { type VideoCardItem } from './components/VideoCard';

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

function videoToCardItem(v: ChannelVideo): VideoCardItem {
  return {
    id: v.id,
    title: v.title,
    duration_seconds: v.duration_seconds,
    view_count: v.view_count,
    created_at: v.created_at,
    thumbnail_url: v.thumbnail_url,
    visibility: v.visibility,
    media_type: 'vod',
    status: v.status,
  };
}

function streamToCardItem(s: ChannelLiveStream): VideoCardItem {
  return {
    id: s.id,
    title: s.title,
    created_at: s.created_at,
    published_at: s.started_at,
    media_type: 'live',
    status: s.status,
  };
}

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
          throw new Error((err as { error?: string }).error || 'channel not found');
        }

        const channelData = (await channelRes.json()) as ChannelData;
        const videosData = (await videosRes.json()) as ChannelVideo[];
        const liveData = (await liveRes.json()) as ChannelLiveStream[];

        if (!cancelled) {
          setChannel(channelData);
          setVideos(videosData);
          setLiveStreams(liveData);
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      }
    }

    load();
    return () => { cancelled = true; };
  }, [userId]);

  if (error) {
    return (
      <main className="app-shell">
        <EmptyState
          icon="generic"
          title="Channel not found"
          description={error}
          action={{ label: 'Go home', href: '/' }}
        />
      </main>
    );
  }

  if (!channel) {
    return (
      <main className="app-shell">
        <div className="loading-state" aria-busy="true" aria-label="Loading channel">
          <div className="spinner" aria-hidden="true" />
          <p>Loading channel…</p>
        </div>
      </main>
    );
  }

  const memberSince = new Date(channel.created_at).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });

  function handleVideoClick(video: VideoCardItem, e: React.MouseEvent<HTMLAnchorElement>) {
    e.preventDefault();
    const path = video.media_type === 'live' ? `/live/watch/${video.id}` : `/watch/${video.id}`;
    window.history.pushState({}, '', path);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }

  return (
    <main className="app-shell">
      <div
        className="channel-banner"
        role="img"
        aria-label={`${channel.display_name} channel banner`}
        style={channel.banner_url ? { backgroundImage: `url(${channel.banner_url})` } : undefined}
      >
        <div className="channel-banner-overlay" aria-hidden="true" />
      </div>

      <div className="channel-header">
        <div className="channel-avatar-wrapper">
          {channel.avatar_url ? (
            <img
              src={channel.avatar_url}
              alt={`${channel.display_name} avatar`}
              className="channel-avatar"
            />
          ) : (
            <div className="channel-avatar-placeholder" aria-hidden="true">
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

      <div className="channel-tabs" role="tablist" aria-label="Channel content">
        <button
          role="tab"
          aria-selected={activeTab === 'videos'}
          aria-controls="tab-panel-videos"
          className={`channel-tab ${activeTab === 'videos' ? 'channel-tab--active' : ''}`}
          onClick={() => setActiveTab('videos')}
        >
          Videos ({videos.length})
        </button>
        <button
          role="tab"
          aria-selected={activeTab === 'live'}
          aria-controls="tab-panel-live"
          className={`channel-tab ${activeTab === 'live' ? 'channel-tab--active' : ''}`}
          onClick={() => setActiveTab('live')}
        >
          Live streams ({liveStreams.length})
        </button>
      </div>

      <div
        id="tab-panel-videos"
        role="tabpanel"
        aria-label="Videos"
        hidden={activeTab !== 'videos'}
      >
        {videos.length === 0 ? (
          <EmptyState icon="video" title="No videos yet" description="This channel hasn't uploaded any videos." />
        ) : (
          <VideoGrid
            videos={videos.map(videoToCardItem)}
            showVisibility
            onVideoClick={handleVideoClick}
            className="channel-video-grid"
          />
        )}
      </div>

      <div
        id="tab-panel-live"
        role="tabpanel"
        aria-label="Live streams"
        hidden={activeTab !== 'live'}
      >
        {liveStreams.length === 0 ? (
          <EmptyState icon="live" title="No live streams yet" description="This channel hasn't gone live yet." />
        ) : (
          <VideoGrid
            videos={liveStreams.map(streamToCardItem)}
            getHref={(v) => `/live/watch/${v.id}`}
            onVideoClick={handleVideoClick}
            className="channel-video-grid"
          />
        )}
      </div>
    </main>
  );
}
