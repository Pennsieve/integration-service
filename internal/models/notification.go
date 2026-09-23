package models

import (
	"encoding/json"
	"time"
)

// Topic is an event category users may subscribe to.
type Topic struct {
	TopicID     int64           `json:"topic_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	Context     json.RawMessage `json:"context,omitempty"`
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

// LastSeen holds a user id and when that user last viewed their
// notifications. NotificationPreferences and UserNotificationsLastSeen both
// embed it instead of declaring their own copies of these two fields, so the
// compiler enforces GET/POST and PATCH continuing to serialize
// notificationsLastSeen identically, as this package's doc comments claim
// they do.
//
// NotificationsLastSeen is a *time.Time so that "never viewed" marshals to
// an explicit JSON null rather than Go's zero time
// (0001-01-01T00:00:00Z), which a client would have to special-case.
//
// NOTE: the JSON fields are camelCase, unlike the snake_case used by the
// rest of this package's models. notificationsLastSeen matches the user
// object served by the Pennsieve API's GET /user, which is where clients
// normally read that value from; the other fields follow the same
// convention for consistency.
type LastSeen struct {
	UserID                int64      `json:"userId"`
	NotificationsLastSeen *time.Time `json:"notificationsLastSeen"`
}

// NotificationPreferences is the body returned by
// GET and POST /notification/user/{userId}: a user's full notification
// preferences — their email/push channel opt-ins plus when they last viewed
// their notifications.
type NotificationPreferences struct {
	LastSeen
	EmailEnabled bool `json:"emailEnabled"`
	PushEnabled  bool `json:"pushEnabled"`
}

// SetNotificationPreferencesRequest is the JSON body accepted by
// POST /notification/user/{userId}. POST fully replaces the stored
// email/push preferences, so both fields are required; it never touches
// notificationsLastSeen (see UpdateNotificationsLastSeenRequest for that).
// The user being updated comes from the path, never the body, so this
// deliberately carries no user id.
//
// Fields are pointers so the handler can reject an absent or null value:
// json.Unmarshal leaves a pointer nil in both cases, so nil after a
// successful unmarshal means "not supplied".
type SetNotificationPreferencesRequest struct {
	EmailEnabled *bool `json:"emailEnabled"`
	PushEnabled  *bool `json:"pushEnabled"`
}

// UserNotificationsLastSeen is the body returned by
// PATCH /notification/user/{userId}: when that user last viewed their
// notifications. It is split out from NotificationPreferences because PATCH
// only ever touches this one field, not the full preferences resource.
type UserNotificationsLastSeen struct {
	LastSeen
}

// UpdateNotificationsLastSeenRequest is the JSON body accepted by
// PATCH /notification/user/{userId}: a partial update that only ever sets
// notificationsLastSeen. The user being updated comes from the path, never
// the body, so this deliberately carries no user id.
//
// The field is a pointer so the handler can reject an absent or null
// timestamp: json.Unmarshal leaves it nil in both cases and errors outright
// on one it cannot parse, so a nil value after a successful unmarshal means
// "not supplied".
type UpdateNotificationsLastSeenRequest struct {
	NotificationsLastSeen *time.Time `json:"notificationsLastSeen"`
}
