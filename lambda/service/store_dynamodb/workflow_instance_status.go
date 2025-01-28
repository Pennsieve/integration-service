package store_dynamodb

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type WorkflowInstanceStatus struct {
	WorkflowInstanceUuid string `dynamodbav:"workflowInstanceUuid"`
	ProcessorUuid        string `dynamodbav:"processorUuid"`
	Status               string `dynamodbav:"status"`
	Timestamp            int    `dynamodbav:"timestamp"`
}

func (s WorkflowInstanceStatus) GetKey() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"workflowInstanceUuid": &types.AttributeValueMemberS{
			Value: s.WorkflowInstanceUuid,
		},
		"processorUuid#timestamp": &types.AttributeValueMemberS{
			Value: fmt.Sprintf("%s#%d", s.ProcessorUuid, s.Timestamp),
		},
	}
}
