-- Whether a topic accepts new subscriptions, and whether a subscription
-- should still be delivered to. Both replace deleting rows: notifications
-- cascade off subscriptions (and subscriptions off topics), so a delete would
-- also erase the user's notification history. Existing rows were all live,
-- so they are backfilled as enabled.
ALTER TABLE topics
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;
