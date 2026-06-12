-- stream_key_suffix stores the last 8 chars of the raw key so users can
-- identify their key without the API ever returning the full raw value again.
ALTER TABLE live_streams ADD COLUMN IF NOT EXISTS stream_key_suffix TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_live_streams_owner_id ON live_streams (owner_id);
CREATE INDEX IF NOT EXISTS idx_live_streams_status ON live_streams (status);
