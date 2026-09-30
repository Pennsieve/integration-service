package notification_matcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
)

// Match returns the subscriptions to topic that event notifies. An event
// that can't supply the topic's required context fields returns an error
// wrapping ErrBadField, which the caller should log and skip; any other
// error (e.g. from the database) is worth retrying.
func Match(ctx context.Context, topic models.Topic, event models.EventMessage) ([]models.Subscription, error) {
	extracted, err := ExtractContext(topic, event)
	if err != nil {
		return nil, err
	}
	eventContext, err := json.Marshal(extracted)
	if err != nil {
		return nil, fmt.Errorf("encode event context: %w", err)
	}
	return db.MatchSubscriptions(ctx, topic.TopicID, eventContext)
}
