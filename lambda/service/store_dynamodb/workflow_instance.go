package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/pennsieve/integration-service/service/models"
)

type WorkflowInstance struct {
	Uuid                  string                             `dynamodbav:"uuid"`
	Name                  string                             `dynamodbav:"name"`
	ComputeNodeUuid       string                             `dynamodbav:"computeNodeUuid"`
	ComputeNodeGatewayUrl string                             `dynamodbav:"computeNodeGatewayUrl"`
	DatasetNodeId         string                             `dynamodbav:"datasetNodeId"`
	PackageIds            []string                           `dynamodbav:"packageIds"`
	Workflow              interface{}                        `dynamodbav:"workflow"`
	WorkflowUuid          string                             `dynamodbav:"workflowUuid"`
	ExecutionOrder        [][]string                         `dynamodbav:"executionOrder"`
	InvocationParams      map[string][]models.ProcessorParam `dynamodbav:"invocationParams"`
	Params                interface{}                        `dynamodbav:"params"`
	OrganizationId        string                             `dynamodbav:"organizationId"`
	AccountId             string                             `dynamodbav:"accountId"`
	Status                string                             `dynamodbav:"status"`
	StartedAt             string                             `dynamodbav:"startedAt"`
	CompletedAt           string                             `dynamodbav:"completedAt"`
}

type WorkflowInstanceKey struct {
	Uuid string `dynamodbav:"uuid"`
}

func (i WorkflowInstance) GetKey() map[string]types.AttributeValue {
	uuid, err := attributevalue.Marshal(i.Uuid)
	if err != nil {
		panic(err)
	}

	return map[string]types.AttributeValue{"uuid": uuid}
}
