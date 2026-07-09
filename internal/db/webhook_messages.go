package db

import (
	"context"
	"fmt"
	"time"

	"github.com/Pennsieve/integration-service/internal/models"
)

// InsertWebhookMessage persists an incoming webhook payload into
// webhooks.messages and returns the assigned serial id.
func InsertWebhookMessage(ctx context.Context, requestID string, payload []byte) (models.IncomingWebhook, error) {
	const q = `
		INSERT INTO webhooks.messages (request_id, payload)
		VALUES ($1, $2)
		RETURNING id, request_id, payload, received_at`

	var rec models.IncomingWebhook
	err := dbPool.QueryRowContext(ctx, q, requestID, payload).Scan(
		&rec.ID,
		&rec.RequestID,
		&rec.Payload,
		&rec.ReceivedAt,
	)
	if err != nil {
		return models.IncomingWebhook{}, fmt.Errorf("insert webhook message: %w", err)
	}
	return rec, nil
}

// RecordSenderRequest increments the fixed-window request counter for
// senderIP and returns the count after recording this request. The window
// resets (count goes back to 1) once it's older than window. The upsert is
// a single atomic statement so concurrent Lambda invocations for the same
// sender can't race past each other and both see a stale count.
func RecordSenderRequest(ctx context.Context, senderIP string, window time.Duration) (int, error) {
	const q = `
		INSERT INTO webhooks.sender_rate_limits (sender_ip, window_start, request_count)
		VALUES ($1, now(), 1)
		ON CONFLICT (sender_ip) DO UPDATE SET
			window_start = CASE
				WHEN webhooks.sender_rate_limits.window_start <= now() - ($2 * interval '1 second')
				THEN now()
				ELSE webhooks.sender_rate_limits.window_start
			END,
			request_count = CASE
				WHEN webhooks.sender_rate_limits.window_start <= now() - ($2 * interval '1 second')
				THEN 1
				ELSE webhooks.sender_rate_limits.request_count + 1
			END
		RETURNING request_count`

	var count int
	err := dbPool.QueryRowContext(ctx, q, senderIP, window.Seconds()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("record sender request: %w", err)
	}
	return count, nil
}
