package store_dynamodb

type WorkflowInstanceStatus struct {
	WorkflowInstanceUuid string `dynamodbav:"workflowInstanceUuid"`
	ProcessorUuid        string `dynamodbav:"processorUuid"`
	Status               string `dynamodbav:"status"`
	Timestamp            int    `dynamodbav:"timestamp"`
}
