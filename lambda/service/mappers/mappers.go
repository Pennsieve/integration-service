package mappers

import (
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func DynamoDBIntegrationToJsonIntegration(dynamoIntegrations []store_dynamodb.WorkflowInstance) []models.WorkflowInstance {
	integrations := []models.WorkflowInstance{}

	for _, a := range dynamoIntegrations {
		integrations = append(integrations, models.WorkflowInstance{
			Uuid: a.Uuid,
			ComputeNode: models.ComputeNode{
				ComputeNodeUuid: a.ComputeNodeUuid,
			},
			DatasetNodeID: a.DatasetNodeId,
			PackageIDs:    a.PackageIds,
			Workflow:      a.Workflow,
			Params:        a.Params,
			StartedAt:     a.StartedAt,
			CompletedAt:   a.CompletedAt,
		})
	}

	return integrations
}
