package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type WorkflowInstanceStatus struct {
	Uuid          string `dynamodbav:"uuid"`
	ProcessorUuid string `dynamodbav:"processorUuid"`
	Status        string `dynamodbav:"status"`
	Timestamp     int    `dynamodbav:"timestamp"`
}

type WorkflowInstanceStatusKey struct {
	Uuid      string `dynamodbav:"uuid"`
	Timestamp int    `dynamodbav:"timestamp"`
}

func (s WorkflowInstanceStatus) GetKey() map[string]types.AttributeValue {
	uuid, err := attributevalue.Marshal(s.Uuid)
	if err != nil {
		panic(err)
	}

	timestamp, err := attributevalue.Marshal(s.Timestamp)
	if err != nil {
		panic(err)
	}

	return map[string]types.AttributeValue{"uuid": uuid, "timestamp": timestamp}
}
