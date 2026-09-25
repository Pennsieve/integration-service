ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS enabled;

ALTER TABLE topics
    DROP COLUMN IF EXISTS enabled;
