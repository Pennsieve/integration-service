package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventMessage_OrganizationEventDecodesDetailWithoutDatasetID(t *testing.T) {
	raw := `{
		"organizationId": "45",
		"eventCategory": "ORGANIZATION",
		"eventType": "DATASET_PUBLISHED_IN_WORKSPACE",
		"eventDetail": {"datasetId": 123, "datasetName": "Sleep EEG"}
	}`
	var e EventMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &e))
	assert.Equal(t, "45", e.OrgID)
	assert.Nil(t, e.DatasetID, "the detail's datasetId must not become the envelope's")
	assert.Equal(t, EventCategoryOrganization, e.Category)
	assert.Equal(t, EventTypeDatasetPublishedInWorkspace, e.Type)
	assert.JSONEq(t, `{"datasetId": 123, "datasetName": "Sleep EEG"}`, string(e.Detail))
}

func TestEventMessage_MissingOrNullDetail(t *testing.T) {
	for name, raw := range map[string]string{
		"missing": `{"organizationId":"45","eventType":"X"}`,
		"null":    `{"organizationId":"45","eventType":"X","eventDetail":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			var e EventMessage
			require.NoError(t, json.Unmarshal([]byte(raw), &e))
			assert.Nil(t, e.Detail)
		})
	}
}

func TestEventMessage_MarshalOmitsDetail(t *testing.T) {
	// Webhook bodies are the marshaled EventMessage; adding eventDetail
	// decoding must not change what webhook receivers get.
	e := EventMessage{OrgID: "45", DatasetID: new(1), Category: "PUBLISHING", Type: "COMPLETE_PUBLICATION",
		Detail: json.RawMessage(`{"doi":"10.1/x"}`)}
	b, err := json.Marshal(e)
	require.NoError(t, err)
	assert.JSONEq(t, `{"organizationId":"45","datasetId":1,"eventCategory":"PUBLISHING","eventType":"COMPLETE_PUBLICATION"}`, string(b))
}
