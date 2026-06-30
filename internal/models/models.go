package models

import "time"

type WebhookCache struct {
	Updated  time.Time
	Webhooks []WebhookRecord
}

type WebhookRecord struct {
	APIURL    string
	EventName string
	DatasetID int
}

type EventMessage struct {
	OrgID    string `json:"organizationId"`
	DataID   int    `json:"datasetId"`
	Category string `json:"eventCategory"`
	Type     string `json:"eventType"`
}

type WebhookMessage struct {
	Messages []EventMessage
	URLs     []string
}

// IncomingWebhook is the stored record for a received webhook message.
type IncomingWebhook struct {
	ID         int64
	RequestID  string
	Payload    []byte
	ReceivedAt time.Time
}

// WebhookResponse is the JSON body returned for every webhook request.
type WebhookResponse struct {
	RequestID  string    `json:"request_id"`
	ReceivedAt time.Time `json:"received_at"`
	Code       int       `json:"code"`
	Message    string    `json:"message"`
}
