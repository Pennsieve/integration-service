package store_dynamodb

type WorkflowInstanceStatus struct {
	Uuid          string `dynamodbav:"uuid"`
	ProcessorUuid string `dynamodbav:"processorUuid"`
	Status        string `dynamodbav:"status"`
	Timestamp     int    `dynamodbav:"timestamp"`
}
