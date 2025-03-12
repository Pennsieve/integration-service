package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type WorkflowInstanceProcessorStatus struct {
	WorkflowInstanceUuid string `dynamodbav:"workflowInstanceUuid"`
	ProcessorUuid        string `dynamodbav:"processorUuid"`
	Status               string `dynamodbav:"status"`
	StartedAt            string `dynamodbav:"startedAt"`
	CompletedAt          string `dynamodbav:"completedAt"`
}

func (s WorkflowInstanceProcessorStatus) GetKey() map[string]types.AttributeValue {
	return WorkflowInstanceProcessorStatusKey(s.WorkflowInstanceUuid, s.ProcessorUuid)
}

func WorkflowInstanceProcessorStatusKey(workflowInstanceUuid string, processorUuid string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"workflowInstanceUuid": &types.AttributeValueMemberS{
			Value: workflowInstanceUuid,
		},
		"processorUuid": &types.AttributeValueMemberS{
			Value: processorUuid,
		},
	}
}
