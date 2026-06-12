ALTER TABLE search_documents
  ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS media_type TEXT NOT NULL DEFAULT 'vod' CHECK (media_type IN ('vod', 'live')),
  ADD COLUMN IF NOT EXISTS duration_seconds INTEGER;

UPDATE search_documents sd
SET owner_id = v.owner_id,
    media_type = 'vod',
    duration_seconds = v.duration_seconds
FROM videos v
WHERE v.id = sd.video_id;

CREATE INDEX IF NOT EXISTS idx_search_documents_owner_id ON search_documents(owner_id);
CREATE INDEX IF NOT EXISTS idx_search_documents_media_type ON search_documents(media_type);
CREATE INDEX IF NOT EXISTS idx_search_documents_duration ON search_documents(duration_seconds);
