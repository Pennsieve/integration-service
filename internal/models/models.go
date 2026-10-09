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

// EventMessage is one changelog event as published to SNS. Producers send
// organizationId and datasetId as quoted strings ("45", "123"), though SNS
// has also been observed to emit datasetId as a JSON number, so both forms
// are accepted.
//
// Not every event carries every field. A missing (or null) field is recorded
// as the zero value, an empty OrgID or a nil DatasetID, rather than failing
// the decode, so one incomplete event can't abort the rest of its batch.
// Consumers decide for themselves whether they need a field: webhooks skip
// an event without a dataset, and notification matching skips it only for
// topics whose context requires one.
//
// Detail is the event-specific eventDetail object, kept raw for whichever
// consumer knows its shape (notification rendering). It is excluded from
// marshaling so webhook deliveries keep their existing body.
type EventMessage struct {
	OrgID     string          `json:"organizationId"`
	DatasetID *int            `json:"datasetId"`
	Category  string          `json:"eventCategory"`
	Type      string          `json:"eventType"`
	Detail    json.RawMessage `json:"-"`
}

// Event categories and types the notification pipeline handles specially.
// EventCategoryOrganization marks organization-level events, which carry no
// datasetId on the envelope even when their detail describes a dataset.
const (
	EventCategoryOrganization            = "ORGANIZATION"
	EventTypeDatasetPublishedInWorkspace = "DATASET_PUBLISHED_IN_WORKSPACE"
)

// DatasetPublishedInWorkspaceDetail is the eventDetail of a
// DATASET_PUBLISHED_IN_WORKSPACE event, emitted by pennsieve-api once
// Discover confirms a dataset publication (first version or revision) is
// live. It holds everything a notification needs to render without a
// follow-up lookup; the organization id is on the envelope, not here.
type DatasetPublishedInWorkspaceDetail struct {
	DatasetID        int    `json:"datasetId"`
	DatasetNodeID    string `json:"datasetNodeId"`
	DatasetName      string `json:"datasetName"`
	PublishedVersion int    `json:"publishedVersion"`
	DOI              string `json:"doi"`
	OwnerUserID      int    `json:"ownerUserId"`
	OwnerName        string `json:"ownerName"`
}

// UnmarshalJSON accepts organizationId and datasetId as either JSON numbers
// or quoted strings, and leaves a missing or null one unset. A value that is
// present but can't be read as its field's type is still an error.
func (e *EventMessage) UnmarshalJSON(data []byte) error {
	var raw struct {
		OrgID     json.RawMessage `json:"organizationId"`
		DatasetID json.RawMessage `json:"datasetId"`
		Category  string          `json:"eventCategory"`
		Type      string          `json:"eventType"`
		Detail    json.RawMessage `json:"eventDetail"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	orgID, err := stringOrNumber(raw.OrgID)
	if err != nil {
		return fmt.Errorf("organizationId: %w", err)
	}
	datasetID, err := optionalInt(raw.DatasetID)
	if err != nil {
		return fmt.Errorf("datasetId: %w", err)
	}

	var detail json.RawMessage
	if !isAbsent(raw.Detail) {
		detail = raw.Detail
	}

	*e = EventMessage{OrgID: orgID, DatasetID: datasetID, Category: raw.Category, Type: raw.Type, Detail: detail}
	return nil
}

// isAbsent reports whether a raw field was missing or explicitly null.
func isAbsent(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

// stringOrNumber reads a JSON string or number as a string, keeping a
// number's literal text, and returns "" for a missing or null field.
func stringOrNumber(raw json.RawMessage) (string, error) {
	if isAbsent(raw) {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return "", fmt.Errorf("cannot unmarshal %s", raw)
	}
	return n.String(), nil
}

// optionalInt reads a JSON number or numeric string as a positive int, and
// returns nil for a missing or null field. Ids are always positive, so zero
// or a negative value is rejected here rather than left for each consumer
// to catch: one that only nil-checks would otherwise build a lookup key
// that silently matches nothing.
func optionalInt(raw json.RawMessage) (*int, error) {
	if isAbsent(raw) {
		return nil, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("cannot unmarshal %s", raw)
		}
		if n, err = strconv.Atoi(s); err != nil {
			return nil, fmt.Errorf("cannot convert %q to int: %w", s, err)
		}
	}
	if n <= 0 {
		return nil, fmt.Errorf("got %d, want a positive integer", n)
	}
	return &n, nil
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
