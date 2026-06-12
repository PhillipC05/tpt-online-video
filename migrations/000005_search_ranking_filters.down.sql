DROP INDEX IF EXISTS idx_search_documents_duration;
DROP INDEX IF EXISTS idx_search_documents_media_type;
DROP INDEX IF EXISTS idx_search_documents_owner_id;

ALTER TABLE search_documents DROP COLUMN IF EXISTS duration_seconds;
ALTER TABLE search_documents DROP COLUMN IF EXISTS media_type;
ALTER TABLE search_documents DROP COLUMN IF EXISTS owner_id;
