package event_parser

import (
	"encoding/json"
	"fmt"

	"github.com/Pennsieve/integration-service/internal/models"
)

func MapEvents(events map[string]interface{}) (map[string][]models.EventMessage, bool, error) {
	mapped := make(map[string][]models.EventMessage)
	forceRefresh := false

	records, ok := events["Records"].([]interface{})
	if !ok {
		return nil, false, fmt.Errorf("invalid event format: records field missing or wrong type")
	}
	for _, r := range records {
		//TODO: We want per-record skip-and-log.
		rec, ok := r.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("record not an object")
		}
		rawBody, ok := rec["body"]
		if !ok {
			return nil, false, fmt.Errorf("record.body missing")
		}
		body, ok := rawBody.(string)
		if !ok {
			return nil, false, fmt.Errorf("record.body not a string")
		}

		var bodyJSON map[string]interface{}
		err := json.Unmarshal([]byte(body), &bodyJSON)
		if err != nil {
			return nil, false, err
		}
		msgStr, ok := bodyJSON["Message"].(string)
		if !ok {
			return nil, false, fmt.Errorf("record.body.Message not a string")
		}
		var msg models.EventMessage
		err = json.Unmarshal([]byte(msgStr), &msg)
		if err != nil {
			return nil, false, err
		}

		mapped[msg.OrgID] = append(mapped[msg.OrgID], msg)

		if msg.Type == "CREATE_DATASET" {
			forceRefresh = true
		}
	}

	return mapped, forceRefresh, nil
}
