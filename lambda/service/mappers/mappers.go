package mappers

import (
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func DynamoDBIntegrationToJsonIntegration(dynamoIntegrations []store_dynamodb.Integration) []models.Integration {
	integrations := []models.Integration{}

	for _, a := range dynamoIntegrations {
		integrations = append(integrations, models.Integration{
			Uuid:          a.Uuid,
			ApplicationID: a.ApplicationId,
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
