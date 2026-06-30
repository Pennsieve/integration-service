package db

import (
	"context"
	"fmt"

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
