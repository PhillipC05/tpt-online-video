ALTER TABLE live_streams DROP COLUMN IF EXISTS chat_locked;
DROP TABLE IF EXISTS live_chat_timeouts;
DROP TABLE IF EXISTS live_chat_bans;
DROP TABLE IF EXISTS live_chat_messages;
