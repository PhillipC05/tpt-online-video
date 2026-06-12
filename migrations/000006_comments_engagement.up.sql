CREATE TABLE IF NOT EXISTS video_likes (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, video_id)
);

CREATE TABLE IF NOT EXISTS comment_likes (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  comment_id UUID NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, comment_id)
);

CREATE INDEX IF NOT EXISTS idx_video_likes_video_id ON video_likes(video_id);
CREATE INDEX IF NOT EXISTS idx_video_likes_user_id ON video_likes(user_id);
CREATE INDEX IF NOT EXISTS idx_comment_likes_comment_id ON comment_likes(comment_id);
CREATE INDEX IF NOT EXISTS idx_comment_likes_user_id ON comment_likes(user_id);

-- Function to refresh video like_count from the likes table
CREATE OR REPLACE FUNCTION refresh_video_like_count()
RETURNS TRIGGER AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    UPDATE videos SET like_count = (SELECT COUNT(*) FROM video_likes WHERE video_id = NEW.video_id) WHERE id = NEW.video_id;
    RETURN NEW;
  ELSIF TG_OP = 'DELETE' THEN
    UPDATE videos SET like_count = (SELECT COUNT(*) FROM video_likes WHERE video_id = OLD.video_id) WHERE id = OLD.video_id;
    RETURN OLD;
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER refresh_video_like_count_insert
AFTER INSERT ON video_likes
FOR EACH ROW EXECUTE FUNCTION refresh_video_like_count();

CREATE TRIGGER refresh_video_like_count_delete
AFTER DELETE ON video_likes
FOR EACH ROW EXECUTE FUNCTION refresh_video_like_count();