CREATE SCHEMA IF NOT EXISTS notifications;

CREATE TABLE IF NOT EXISTS notifications.topics (
    topic_id    SERIAL      PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    description TEXT,
    context     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notifications.subscriptions (
    subscription_id SERIAL      PRIMARY KEY,
    user_id         INTEGER     NOT NULL REFERENCES pennsieve.users (id) ON DELETE CASCADE,
    topic_id        INTEGER     NOT NULL REFERENCES notifications.topics (topic_id) ON DELETE CASCADE,
    context         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, topic_id, context)
);

CREATE INDEX IF NOT EXISTS idx_notifications_subscriptions_user_id  ON notifications.subscriptions (user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_subscriptions_topic_id ON notifications.subscriptions (topic_id);

CREATE TABLE IF NOT EXISTS notifications.notifications (
    notification_id SERIAL      PRIMARY KEY,
    subscription_id INTEGER     NOT NULL REFERENCES notifications.subscriptions (subscription_id) ON DELETE CASCADE,
    title           TEXT        NOT NULL,
    message         TEXT        NOT NULL,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_notifications_subscription_id ON notifications.notifications (subscription_id);
CREATE INDEX IF NOT EXISTS idx_notifications_notifications_created_at      ON notifications.notifications (created_at DESC);

CREATE TABLE IF NOT EXISTS notifications.messages (
    message_id      SERIAL      PRIMARY KEY,
    from_user       INTEGER     NOT NULL REFERENCES pennsieve.users (id),
    to_user         INTEGER     NOT NULL REFERENCES pennsieve.users (id),
    notification_id INTEGER     REFERENCES notifications.notifications (notification_id) ON DELETE CASCADE,
    content         TEXT        NOT NULL,
    "timestamp"     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_messages_notification_id ON notifications.messages (notification_id);
CREATE INDEX IF NOT EXISTS idx_notifications_messages_from_user       ON notifications.messages (from_user);
CREATE INDEX IF NOT EXISTS idx_notifications_messages_to_user         ON notifications.messages (to_user);

CREATE TABLE IF NOT EXISTS notifications.user_notifications (
    user_notification_id SERIAL      PRIMARY KEY,
    user_id              INTEGER     NOT NULL REFERENCES pennsieve.users (id) ON DELETE CASCADE,
    notification_id      INTEGER     NOT NULL REFERENCES notifications.notifications (notification_id) ON DELETE CASCADE,
    status                TEXT        NOT NULL DEFAULT 'UNREAD' CHECK (status IN ('READ', 'UNREAD')),
    delivered_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at               TIMESTAMPTZ,
    UNIQUE (user_id, notification_id)
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_notifications_user_id         ON notifications.user_notifications (user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_notifications_notification_id ON notifications.user_notifications (notification_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_notifications_status         ON notifications.user_notifications (status);

CREATE TABLE IF NOT EXISTS notifications.preferences (
    user_id       INTEGER PRIMARY KEY REFERENCES pennsieve.users (id) ON DELETE CASCADE,
    email_enabled BOOLEAN NOT NULL DEFAULT true,
    sms_enabled   BOOLEAN NOT NULL DEFAULT false,
    push_enabled  BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS notifications.notification_audit (
    audit_id        SERIAL      PRIMARY KEY,
    notification_id INTEGER     NOT NULL REFERENCES notifications.notifications (notification_id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    "timestamp"     TIMESTAMPTZ NOT NULL DEFAULT now(),
    details         JSONB
);

CREATE INDEX IF NOT EXISTS idx_notifications_notification_audit_notification_id ON notifications.notification_audit (notification_id);
