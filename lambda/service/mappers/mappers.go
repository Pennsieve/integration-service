package mappers

import (
	"encoding/json"

	"github.com/pennsieve/integration-service/service/log_retriever"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func DynamoDBIntegrationToJsonIntegration(dynamoIntegrations []store_dynamodb.WorkflowInstance) []models.WorkflowInstance {
	integrations := []models.WorkflowInstance{}

	for _, a := range dynamoIntegrations {
		integrations = append(integrations, models.WorkflowInstance{
			Uuid: a.Uuid,
			Name: a.Name,
			ComputeNode: models.ComputeNode{
				ComputeNodeUuid:       a.ComputeNodeUuid,
				ComputeNodeGatewayUrl: a.ComputeNodeGatewayUrl,
			},
			DatasetNodeID: a.DatasetNodeId,
			PackageIDs:    a.PackageIds,
			Workflow:      a.Workflow,
			Params:        a.Params,
			Status:        a.Status,
			StartedAt:     a.StartedAt,
			CompletedAt:   a.CompletedAt,
		})
	}

	return integrations
}

func ServiceResponseToAuxiliaryResponse(resp []byte) (log_retriever.ProcessorLogPayload, error) {
	var m log_retriever.ProcessorLogPayload
	if err := json.Unmarshal(resp, &m); err != nil {
		return log_retriever.ProcessorLogPayload{}, err
	}

	return m, nil
}

func ExtractWorkflow(workflow interface{}) ([]models.WorkflowProcessor, error) {
	workflowBytes, err := json.Marshal(workflow)
	if err != nil {
		return nil, err
	}

	var wf []models.WorkflowProcessor
	err = json.Unmarshal(workflowBytes, &wf)
	if err != nil {
		return nil, err
	}

	return wf, nil
}
