DROP INDEX IF EXISTS idx_live_streams_dvr_cleanup;
ALTER TABLE live_streams DROP COLUMN IF EXISTS dvr_cleaned_at;
