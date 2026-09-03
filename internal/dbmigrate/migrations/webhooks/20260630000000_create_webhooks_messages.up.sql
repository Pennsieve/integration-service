CREATE TABLE IF NOT EXISTS messages (
    id          SERIAL      PRIMARY KEY,
    request_id  UUID        NOT NULL DEFAULT gen_random_uuid(),
    payload     JSONB       NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_messages_request_id  ON messages (request_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_messages_received_at ON messages (received_at DESC);
