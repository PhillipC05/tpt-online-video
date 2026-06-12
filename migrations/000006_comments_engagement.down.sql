DROP TRIGGER IF EXISTS refresh_video_like_count_insert ON video_likes;
DROP TRIGGER IF EXISTS refresh_video_like_count_delete ON video_likes;
DROP FUNCTION IF EXISTS refresh_video_like_count;

DROP TABLE IF EXISTS comment_likes;
DROP TABLE IF EXISTS video_likes;