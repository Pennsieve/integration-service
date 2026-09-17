-- When the user last viewed their notifications. NULL means never viewed,
-- so the column is deliberately nullable with no default: backfilling
-- existing rows with now() would claim every user had just read everything.
ALTER TABLE preferences
    ADD COLUMN IF NOT EXISTS notifications_last_seen TIMESTAMPTZ;
