-- DESTRUCTIVE: this drops opt-out state with no way to recover it. Once
-- users have disabled subscriptions (or admins have disabled topics), rolling
-- this back loses which rows were off. If the columns are later re-added by
-- the up migration, every row is backfilled to enabled = true, which silently
-- turns delivery back on for users who had explicitly opted out.
--
-- Before running this against an environment where the flags have been used,
-- snapshot the disabled rows so they can be re-applied after re-migrating:
--
--   CREATE TABLE notifications.enabled_flags_backup AS
--       SELECT 'subscription' AS entity, subscription_id AS id
--       FROM notifications.subscriptions WHERE NOT enabled
--       UNION ALL
--       SELECT 'topic', topic_id FROM notifications.topics WHERE NOT enabled;
--
-- The snapshot is left to the operator rather than done here so that a
-- rollback in a fresh or test environment stays a plain column drop.
ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS enabled;

ALTER TABLE topics
    DROP COLUMN IF EXISTS enabled;
