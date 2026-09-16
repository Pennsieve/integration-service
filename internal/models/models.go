package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

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
	OrgID     string `json:"organizationId"`
	DatasetID int    `json:"datasetId"`
	Category  string `json:"eventCategory"`
	Type      string `json:"eventType"`
}

// UnmarshalJSON handles datasetId sent as either a JSON number or a quoted
// string (e.g. "2252"), which SNS has been observed to emit.
func (e *EventMessage) UnmarshalJSON(data []byte) error {
	var raw struct {
		OrgID     string          `json:"organizationId"`
		DatasetID json.RawMessage `json:"datasetId"`
		Category  string          `json:"eventCategory"`
		Type      string          `json:"eventType"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.OrgID = raw.OrgID
	e.Category = raw.Category
	e.Type = raw.Type

	var n int
	if err := json.Unmarshal(raw.DatasetID, &n); err == nil {
		e.DatasetID = n
		return nil
	}
	var s string
	if err := json.Unmarshal(raw.DatasetID, &s); err != nil {
		return fmt.Errorf("datasetId: cannot unmarshal %s", raw.DatasetID)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("datasetId: cannot convert %q to int: %w", s, err)
	}
	e.DatasetID = n
	return nil
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
