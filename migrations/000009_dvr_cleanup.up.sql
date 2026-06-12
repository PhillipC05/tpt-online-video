-- Track when the DVR cleaner removed disk segments for an ended stream.
-- NULL means not yet cleaned (or DVR was disabled).
ALTER TABLE live_streams ADD COLUMN IF NOT EXISTS dvr_cleaned_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_live_streams_dvr_cleanup
    ON live_streams (status, dvr_enabled, dvr_cleaned_at)
    WHERE status = 'ended' AND dvr_enabled = true AND dvr_cleaned_at IS NULL;
