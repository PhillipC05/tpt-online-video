-- Persisted live chat messages
CREATE TABLE IF NOT EXISTS live_chat_messages (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stream_id    UUID NOT NULL REFERENCES live_streams(id) ON DELETE CASCADE,
  user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
  display_name TEXT NOT NULL DEFAULT 'Anonymous',
  body         TEXT NOT NULL,
  deleted      BOOLEAN NOT NULL DEFAULT FALSE,
  deleted_by   UUID REFERENCES users(id) ON DELETE SET NULL,
  deleted_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lcm_stream_created ON live_chat_messages(stream_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_lcm_user_id ON live_chat_messages(user_id);

-- Per-stream permanent chat bans
CREATE TABLE IF NOT EXISTS live_chat_bans (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stream_id  UUID NOT NULL REFERENCES live_streams(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  banned_by  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason     TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (stream_id, user_id)
);

-- Per-stream temporary chat timeouts (mutes)
CREATE TABLE IF NOT EXISTS live_chat_timeouts (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stream_id        UUID NOT NULL REFERENCES live_streams(id) ON DELETE CASCADE,
  user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  timed_out_by     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  duration_seconds INT NOT NULL DEFAULT 300,
  expires_at       TIMESTAMPTZ NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (stream_id, user_id)
);

-- Chat lock flag on live_streams
ALTER TABLE live_streams ADD COLUMN IF NOT EXISTS chat_locked BOOLEAN NOT NULL DEFAULT FALSE;
