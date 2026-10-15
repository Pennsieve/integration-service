-- DESTRUCTIVE: drops every per-subscription channel override. After this,
-- those subscriptions deliver on the user's preferences again, and
-- re-running the up migration leaves them all NULL (no override).
ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_notification_channels_check;

ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS notification_channels;
