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

// Notification records that an event occurred on a topic.
type Notification struct {
	NotificationID int64           `json:"notification_id"`
	TopicID        int64           `json:"topic_id"`
	SenderID       int64           `json:"sender_id"`
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
