ALTER TABLE notifications.subscriptions ADD CONSTRAINT notifications_subscriptions_context_key UNIQUE (context);

ALTER TABLE notifications.notifications DROP COLUMN sender_id;
