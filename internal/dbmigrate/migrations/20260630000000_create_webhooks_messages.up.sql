CREATE SCHEMA IF NOT EXISTS webhooks;

CREATE TABLE IF NOT EXISTS webhooks.messages (
    id          SERIAL      PRIMARY KEY,
    request_id  UUID        NOT NULL DEFAULT gen_random_uuid(),
    payload     JSONB       NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_messages_request_id  ON webhooks.messages (request_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_messages_received_at ON webhooks.messages (received_at DESC);
