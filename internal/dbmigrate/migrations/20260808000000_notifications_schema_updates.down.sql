ALTER TABLE notifications.notifications ADD COLUMN sender_id INTEGER NOT NULL REFERENCES pennsieve.users (id);

ALTER TABLE notifications.subscriptions DROP CONSTRAINT notifications_subscriptions_context_key;
