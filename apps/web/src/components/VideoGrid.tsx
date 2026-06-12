import VideoCard, { VideoCardItem } from './VideoCard';

type Props = {
  videos: VideoCardItem[];
  getHref?: (video: VideoCardItem) => string;
  onVideoClick?: (video: VideoCardItem, e: React.MouseEvent<HTMLAnchorElement>) => void;
  showVisibility?: boolean;
  className?: string;
};

export default function VideoGrid({
  videos,
  getHref = (v) => `/watch/${v.id}`,
  onVideoClick,
  showVisibility,
  className = 'video-grid',
}: Props) {
  return (
    <section className={className} aria-label="Video list">
      {videos.map((video) => (
        <VideoCard
          key={video.id}
          video={video}
          href={getHref(video)}
          onClick={onVideoClick ? (e) => onVideoClick(video, e) : undefined}
          showVisibility={showVisibility}
        />
      ))}
    </section>
  );
}
