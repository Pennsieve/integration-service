package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type WorkflowInstance struct {
	Uuid                  string      `dynamodbav:"uuid"`
	ComputeNodeUuid       string      `dynamodbav:"computeNodeUuid"`
	ComputeNodeGatewayUrl string      `dynamodbav:"computeNodeGatewayUrl"`
	DatasetNodeId         string      `dynamodbav:"datasetNodeId"`
	PackageIds            []string    `dynamodbav:"packageIds"`
	Workflow              interface{} `dynamodbav:"workflow"`
	Params                interface{} `dynamodbav:"params"`
	OrganizationId        string      `dynamodbav:"organizationId"`
	StartedAt             string      `dynamodbav:"startedAt"`
	CompletedAt           string      `dynamodbav:"completedAt"`
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