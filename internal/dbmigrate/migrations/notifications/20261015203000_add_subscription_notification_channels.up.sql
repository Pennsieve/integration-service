-- Per-subscription notification channels, overriding the user-wide
-- email/push/sms opt-ins on preferences for this one subscription. NULL (the
-- default, and every existing row) means "no override": deliveries for the
-- subscription fall back to the user's preferences. When set it must carry
-- exactly the three channel keys, each a boolean, so the delivery path never
-- has to guess what a missing or misspelled key means.
--
-- The COALESCE matters: a missing key makes "-> 'sms'" NULL, so the AND
-- chain evaluates to NULL rather than false, and a CHECK lets NULL through.
ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS notification_channels JSONB;

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_notification_channels_check;

ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_notification_channels_check CHECK (
        notification_channels IS NULL OR COALESCE(
            jsonb_typeof(notification_channels) = 'object'
            AND jsonb_typeof(notification_channels -> 'email') = 'boolean'
            AND jsonb_typeof(notification_channels -> 'push')  = 'boolean'
            AND jsonb_typeof(notification_channels -> 'sms')   = 'boolean'
            AND notification_channels - ARRAY['email', 'push', 'sms'] = '{}'::jsonb,
            false
        )
    );
