package models

import (
	"encoding/json"
	"time"
)

// Topic is an event category users may subscribe to.
type Topic struct {
	TopicID     int64     `json:"topic_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Subscription represents a user's interest in a topic.
type Subscription struct {
	SubscriptionID int64           `json:"subscription_id"`
	UserID         int64           `json:"user_id"`
	TopicID        int64           `json:"topic_id"`
	Context        json.RawMessage `json:"context,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// SubscribeRequest is the optional JSON body accepted when creating a
// subscription.
type SubscribeRequest struct {
	Context json.RawMessage `json:"context,omitempty"`
}

// Notification records that an event occurred for a subscription.
type Notification struct {
	NotificationID int64           `json:"notification_id"`
	SubscriptionID int64           `json:"subscription_id"`
	Title          string          `json:"title"`
	Message        string          `json:"message"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// NotificationErrorResponse is the JSON body returned for failed
// notification/subscription API requests.
type NotificationErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// UserNotificationsLastSeen is the body returned by
// GET and POST /notification/user/{userId}: when that user last viewed their
// notifications.
//
// NotificationsLastSeen is a *time.Time so that "never viewed" marshals to
// an explicit JSON null rather than Go's zero time
// (0001-01-01T00:00:00Z), which a client would have to special-case.
//
// NOTE: the JSON fields are camelCase, unlike the snake_case used by the
// rest of this package's models. They match the user object served by the
// Pennsieve API's GET /user, which is where clients normally read this value
// from; keeping the two spellings identical means one field name to parse.
type UserNotificationsLastSeen struct {
	UserID                int64      `json:"userId"`
	NotificationsLastSeen *time.Time `json:"notificationsLastSeen"`
}

// SetNotificationsLastSeenRequest is the JSON body accepted by
// POST /notification/user/{userId}. The user being updated comes from the
// path, never the body, so this deliberately carries no user id.
//
// The field is a pointer so the handler can reject an absent or null
// timestamp: json.Unmarshal leaves it nil in both cases and errors outright
// on one it cannot parse, so a nil value after a successful unmarshal means
// "not supplied".
type SetNotificationsLastSeenRequest struct {
	NotificationsLastSeen *time.Time `json:"notificationsLastSeen"`
}
