package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Integration struct {
	Uuid            string      `dynamodbav:"uuid"`
	ComputeNodeUuid string      `dynamodbav:"computeNodeUuid"`
	ApplicationId   int64       `dynamodbav:"applicationId"`
	DatasetNodeId   string      `dynamodbav:"datasetNodeId"`
	PackageIds      []string    `dynamodbav:"packageIds"`
	Workflow        interface{} `dynamodbav:"workflow"`
	Params          interface{} `dynamodbav:"params"`
	OrganizationId  string      `dynamodbav:"organizationId"`
	Start           string      `dynamodbav:"start"`
	End             string      `dynamodbav:"end"`
}

func (i Integration) GetKey() map[string]types.AttributeValue {
	uuid, err := attributevalue.Marshal(i.Uuid)
	if err != nil {
		panic(err)
	}

	return map[string]types.AttributeValue{"uuid": uuid}
}
