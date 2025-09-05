package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Workflow struct {
	Uuid           string      `dynamodbav:"uuid"`
	Name           string      `dynamodbav:"name"`
	Description    string      `dynamodbav:"description"`
	Processors     interface{} `dynamodbav:"processors"`
	OrganizationId string      `dynamodbav:"organizationId"`
	Dag            interface{} `dynamodbav:"dag"`
	ExecutionOrder [][]string  `dynamodbav:"executionOrder"`
	CreatedAt      string      `dynamodbav:"createdAt"`
	CreatedBy      string      `dynamodbav:"createdBy"`
}

type WorkflowKey struct {
	Uuid string `dynamodbav:"uuid"`
}

func (i Workflow) GetKey() map[string]types.AttributeValue {
	uuid, err := attributevalue.Marshal(i.Uuid)
	if err != nil {
		panic(err)
	}

	return map[string]types.AttributeValue{"uuid": uuid}
}
