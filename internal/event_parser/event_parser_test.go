package event_parser

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.Equal(t, new(1), mapped["org1"][0].DatasetID)
}

func TestMapEvents_ForceRefreshOnCreateDataset(t *testing.T) {
	events := sqsEvent(
		map[string]interface{}{"organizationId": "org1", "datasetId": 1, "eventCategory": "DATASET", "eventType": "CREATE_DATASET"},
	)

	_, forceRefresh, err := MapEvents(events)
	require.NoError(t, err)
	assert.True(t, forceRefresh, "CREATE_DATASET must force a cache refresh")
}

func TestMapEvents_DatasetIdAsString(t *testing.T) {
	// SNS has been observed to emit datasetId as a quoted string.
	events := sqsEvent(
		map[string]interface{}{"organizationId": "org1", "datasetId": "2252", "eventCategory": "FILES", "eventType": "CREATE_PACKAGE"},
	)

	mapped, _, err := MapEvents(events)
	require.NoError(t, err)
	assert.Equal(t, new(2252), mapped["org1"][0].DatasetID)
}

func TestMapEvents_MissingDatasetIdIsRecordedNotRejected(t *testing.T) {
	// UPDATE_README-style events can arrive without a datasetId; that must
	// not fail the decode, let alone the batch.
	events := sqsEvent(
		map[string]interface{}{"organizationId": "45", "eventType": "UPDATE_README"},
		map[string]interface{}{"organizationId": "45", "datasetId": nil, "eventType": "UPDATE_README"},
	)

	mapped, _, err := MapEvents(events)
	require.NoError(t, err)
	require.Len(t, mapped["45"], 2)
	assert.Nil(t, mapped["45"][0].DatasetID)
	assert.Nil(t, mapped["45"][1].DatasetID)
}

func TestMapEvents_OrganizationIdAsNumber(t *testing.T) {
	events := sqsEvent(
		map[string]interface{}{"organizationId": 45, "datasetId": "123", "eventType": "UPDATE_README"},
	)

	mapped, _, err := MapEvents(events)
	require.NoError(t, err)
	require.Len(t, mapped["45"], 1)
	assert.Equal(t, new(123), mapped["45"][0].DatasetID)
}

func TestMapEvents_SkipsBadRecordsAndKeepsTheRest(t *testing.T) {
	good := sqsEvent(
		map[string]interface{}{"organizationId": "org1", "datasetId": 1, "eventCategory": "FILES", "eventType": "UPLOAD"},
	)["Records"].([]interface{})[0]

	events := map[string]interface{}{"Records": []interface{}{
		"not an object",
		map[string]interface{}{"messageId": "no-body"},
		map[string]interface{}{"body": 123},
		map[string]interface{}{"body": "{not json"},
		map[string]interface{}{"body": `{"Message": 5}`},
		map[string]interface{}{"body": `{"Message": "{\"datasetId\": \"abc\"}"}`},
		good,
	}}

	mapped, _, err := MapEvents(events)
	require.NoError(t, err, "a bad record must not fail the batch")
	require.Len(t, mapped, 1)
	assert.Len(t, mapped["org1"], 1)
}

func TestMapEvents_RejectsMalformedEnvelope(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"missing Records":    {"NotRecords": []interface{}{}},
		"records wrong type": {"Records": "nope"},
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := MapEvents(ev)
			assert.Error(t, err)
		})
	}
}
