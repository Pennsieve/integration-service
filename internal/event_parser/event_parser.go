package event_parser

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Pennsieve/integration-service/internal/models"
)

// SkippedRecordMarker begins the log line for every record MapEvents skips.
// A CloudWatch metric filter counts it (terraform/cloudwatch.tf) and alarms
// on a sustained rate, standing in for the DLQ alarm that used to catch bad
// records when they failed the batch. Changing it breaks that alarm.
const SkippedRecordMarker = "SKIPPED_EVENT_RECORD"

// MapEvents decodes the SNS-wrapped events in an SQS batch and groups them
// by organization. It also reports whether any event should force a webhook
// cache refresh.
//
// Only a malformed batch envelope is an error. A record that can't be
// decoded is logged and skipped, so one bad record doesn't fail the whole
// batch: a returned error makes Lambda retry every record in the batch until
// it lands in the DLQ, taking the good records with it. Skipped records never
// reach the DLQ, so the skip log line is what alerting watches.
func MapEvents(events map[string]interface{}) (map[string][]models.EventMessage, bool, error) {
	mapped := make(map[string][]models.EventMessage)
	forceRefresh := false

	records, ok := events["Records"].([]interface{})
	if !ok {
		return nil, false, fmt.Errorf("invalid event format: records field missing or wrong type")
	}
	for i, r := range records {
		msg, err := decodeRecord(r)
		if err != nil {
			log.Printf("WARN %s index=%d messageId=%s: %v", SkippedRecordMarker, i, recordMessageID(r), err)
			continue
		}

		mapped[msg.OrgID] = append(mapped[msg.OrgID], msg)

		if msg.Type == "CREATE_DATASET" {
			forceRefresh = true
		}
	}

	return mapped, forceRefresh, nil
}

// decodeRecord unwraps one SQS record's body (an SNS notification) and
// decodes the event in its Message field.
func decodeRecord(r interface{}) (models.EventMessage, error) {
	rec, ok := r.(map[string]interface{})
	if !ok {
		return models.EventMessage{}, fmt.Errorf("record not an object")
	}
	rawBody, ok := rec["body"]
	if !ok {
		return models.EventMessage{}, fmt.Errorf("record.body missing")
	}
	body, ok := rawBody.(string)
	if !ok {
		return models.EventMessage{}, fmt.Errorf("record.body not a string")
	}

	var bodyJSON map[string]interface{}
	if err := json.Unmarshal([]byte(body), &bodyJSON); err != nil {
		return models.EventMessage{}, fmt.Errorf("record.body: %w", err)
	}
	msgStr, ok := bodyJSON["Message"].(string)
	if !ok {
		return models.EventMessage{}, fmt.Errorf("record.body.Message not a string")
	}
	var msg models.EventMessage
	if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
		return models.EventMessage{}, fmt.Errorf("record.body.Message: %w", err)
	}
	return msg, nil
}

// recordMessageID returns the SQS messageId of a record, for logging, or
// "no messageId" when the record doesn't carry one.
func recordMessageID(r interface{}) string {
	if rec, ok := r.(map[string]interface{}); ok {
		if id, ok := rec["messageId"].(string); ok {
			return id
		}
	}
	return "no messageId"
}
