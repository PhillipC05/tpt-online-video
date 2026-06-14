import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import VideoCard, { VideoCardItem } from '../components/VideoCard';

const base: VideoCardItem = {
  id: 'vid-1',
  title: 'My Test Video',
  owner_display_name: 'Alice',
  view_count: 1234,
  duration_seconds: 185,
  media_type: 'vod',
};

describe('VideoCard', () => {
  it('renders title and owner', () => {
    render(<VideoCard video={base} href="/watch/vid-1" />);
    expect(screen.getByText('My Test Video')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('formats duration correctly', () => {
    render(<VideoCard video={base} href="/watch/vid-1" />);
    expect(screen.getByText('3:05')).toBeInTheDocument();
  });

  it('formats view count with locale separator', () => {
    render(<VideoCard video={base} href="/watch/vid-1" />);
    expect(screen.getByText('1,234 views')).toBeInTheDocument();
  });

  it('renders LIVE badge for live media type', () => {
    render(<VideoCard video={{ ...base, media_type: 'live' }} href="/live/watch/vid-1" />);
    expect(screen.getByText('LIVE')).toBeInTheDocument();
  });

  it('shows visibility badge when showVisibility=true and visibility is not public', () => {
    render(
      <VideoCard
        video={{ ...base, visibility: 'unlisted' }}
        href="/watch/vid-1"
        showVisibility
      />
    );
    expect(screen.getByText('unlisted')).toBeInTheDocument();
  });

  it('hides visibility badge when visibility is public', () => {
    render(
      <VideoCard
        video={{ ...base, visibility: 'public' }}
        href="/watch/vid-1"
        showVisibility
      />
    );
    expect(screen.queryByText('public')).not.toBeInTheDocument();
  });

  it('calls onClick when clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn((e: React.MouseEvent) => e.preventDefault());
    render(<VideoCard video={base} href="/watch/vid-1" onClick={onClick} />);
    await user.click(screen.getByRole('link'));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('has accessible label', () => {
    render(<VideoCard video={base} href="/watch/vid-1" />);
    expect(screen.getByRole('link', { name: /watch my test video/i })).toBeInTheDocument();
  });
});
