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
