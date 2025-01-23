package mappers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/mappers"
	"github.com/stretchr/testify/assert"
)

func TestServiceResponseToAuxiliaryResponse(t *testing.T) {
	resp := []byte(`{"messages": [{"timestamp": 1734545552324, "message": "start of processing","ingestionTime": 1734545557240}]}`)

	result, _ := mappers.ServiceResponseToAuxiliaryResponse(resp)
	assert.Equal(t, "2024-12-18 18:12:32 +0000 UTC", result.Messages[0].UtcLocationTime)
	assert.Equal(t, "2024-12-18T18:12:32Z", result.Messages[0].UtcTime)
}

func TestExtractWorkflow(t *testing.T) {
    workflowBytes := []byte(`[
        {
            "uuid": "801a4665-ba47-4053-9071-8503efb063ca",
            "applicationId": "arn:aws:ecs:us-east-1:1234567891011:task-definition/processor-pre-dev:1",
            "applicationContainerName": "processor-pre-ttl-sync-537996532276-dev",
            "applicationType": "preprocessor",
            "params": {},
            "commandArguments": []
        },
        {
            "uuid": "6f412010-c9c7-46ac-b7ce-aed3e06277c8",
            "applicationId": "arn:aws:ecs:us-east-1:1234567891011:task-definition/processor-main-dev:1",
            "applicationContainerName": "ttl-sync-processor-537996532276-dev",
            "applicationType": "processor",
            "params": {},
            "commandArguments": []
        },
        {
            "uuid": "a4c54c4c-4132-4e64-997c-722b79d96dc7",
            "applicationId": "arn:aws:ecs:us-east-1:1234567891011:task-definition/processor-post-dev:1",
            "applicationContainerName": "processor-post-timeseries-537996532276-dev",
            "applicationType": "postprocessor",
            "params": {},
            "commandArguments": []
        }
    ]`)

	var workflow interface{}
    err := json.Unmarshal(workflowBytes, &workflow)
    assert.NoError(t, err)

	result, err := mappers.ExtractWorkflow(workflow)
    assert.NoError(t, err)
    assert.Len(t, result, 3)
    for _, p := range(result) {
        _, err = uuid.Parse(p.Uuid)
        assert.NoError(t, err)
    }
}
