package event_parser

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sqsEvent builds the nested SQS->SNS envelope the Lambda actually receives:
// the outer event has Records[].body (a JSON string), whose "Message" field is
// itself a JSON string holding the real event payload.
func sqsEvent(messages ...map[string]interface{}) map[string]interface{} {
	records := make([]interface{}, 0, len(messages))
	for _, m := range messages {
		msgStr, _ := json.Marshal(m)
		body, _ := json.Marshal(map[string]interface{}{"Message": string(msgStr)})
		records = append(records, map[string]interface{}{"body": string(body)})
	}
	return map[string]interface{}{"Records": records}
}

func TestMapEvents_GroupsByOrg(t *testing.T) {
	events := sqsEvent(
		map[string]interface{}{"organizationId": "org1", "datasetId": 1, "eventCategory": "FILES", "eventType": "UPLOAD"},
		map[string]interface{}{"organizationId": "org1", "datasetId": 2, "eventCategory": "FILES", "eventType": "UPLOAD"},
		map[string]interface{}{"organizationId": "org2", "datasetId": 3, "eventCategory": "FILES", "eventType": "UPLOAD"},
	)

	mapped, forceRefresh, err := MapEvents(events)
	require.NoError(t, err)
	assert.False(t, forceRefresh)
	assert.Len(t, mapped["org1"], 2)
	assert.Len(t, mapped["org2"], 1)
	assert.Equal(t, 1, mapped["org1"][0].DataID)
}

func TestMapEvents_ForceRefreshOnCreateDataset(t *testing.T) {
	events := sqsEvent(
		map[string]interface{}{"organizationId": "org1", "datasetId": 1, "eventCategory": "DATASET", "eventType": "CREATE_DATASET"},
	)

	_, forceRefresh, err := MapEvents(events)
	require.NoError(t, err)
	assert.True(t, forceRefresh, "CREATE_DATASET must force a cache refresh")
}

func TestMapEvents_RejectsMalformedEnvelope(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"missing Records":   {"NotRecords": []interface{}{}},
		"records wrong type": {"Records": "nope"},
		"body not a string": {"Records": []interface{}{map[string]interface{}{"body": 123}}},
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := MapEvents(ev)
			assert.Error(t, err)
		})
	}
}
