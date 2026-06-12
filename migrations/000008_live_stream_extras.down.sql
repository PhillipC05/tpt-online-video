DROP INDEX IF EXISTS idx_live_streams_status;
DROP INDEX IF EXISTS idx_live_streams_owner_id;
ALTER TABLE live_streams DROP COLUMN IF EXISTS stream_key_suffix;
