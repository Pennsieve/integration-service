CREATE TABLE IF NOT EXISTS webhooks.sender_rate_limits (
    sender_ip     TEXT        PRIMARY KEY,
    window_start  TIMESTAMPTZ NOT NULL,
    request_count INTEGER     NOT NULL
);
