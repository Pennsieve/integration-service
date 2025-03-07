package store_dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/pennsieve/integration-service/service/models"
)

type WorkflowInstanceProcessorStatusDBStore interface {
	GetAll(context.Context, string) ([]WorkflowInstanceProcessorStatus, error)
	Put(context.Context, string, string, models.WorkflowInstanceStatusEvent) error
	SetStatus(context.Context, string, string, models.WorkflowInstanceStatusEvent) error
}

type WorkflowInstanceProcessorStatusDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewWorkflowInstanceProcessorStatusDatabaseStore(db *dynamodb.Client, tableName string) WorkflowInstanceProcessorStatusDBStore {
	return &WorkflowInstanceProcessorStatusDatabaseStore{db, tableName}
}

func (r *WorkflowInstanceProcessorStatusDatabaseStore) GetAll(ctx context.Context, workflowInstanceUuid string) ([]WorkflowInstanceProcessorStatus, error) {
	result, err := r.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.TableName),
		KeyConditionExpression: aws.String("workflowInstanceUuid = :workflowInstanceUuid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workflowInstanceUuid": &types.AttributeValueMemberS{Value: workflowInstanceUuid},
		},
	})

	if err != nil {
		return []WorkflowInstanceProcessorStatus{}, fmt.Errorf("error fetching workflow instance processor statuses: %w", err)
	}

	var items []WorkflowInstanceProcessorStatus
	err = attributevalue.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		return []WorkflowInstanceProcessorStatus{}, fmt.Errorf("error unmarshalling workflow instance processor statuses: %w", err)
	}

	return items, nil
}

func (r *WorkflowInstanceProcessorStatusDatabaseStore) SetStatus(ctx context.Context, workflowInstanceUuid string, processorUuid string, event models.WorkflowInstanceStatusEvent) error {
	updateExpression := "SET #status = :status"
	expressionAttributeNames := map[string]string{"#status": "status"}
	expressionAttributeValues := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: event.Status},
	}

	if event.Status == models.WorkflowInstanceStatusStarted {
		updateExpression += ", #startedAt = :startedAt"
		expressionAttributeNames["#startedAt"] = "startedAt"
		expressionAttributeValues[":startedAt"] = &types.AttributeValueMemberS{Value: time.Unix(int64(event.Timestamp), 0).Format(time.RFC3339)}
	} else if models.IsEndStateWorkflowInstanceStatus(event.Status) {
		updateExpression += ", #completedAt = :completedAt"
		expressionAttributeNames["#completedAt"] = "completedAt"
		expressionAttributeValues[":completedAt"] = &types.AttributeValueMemberS{Value: time.Unix(int64(event.Timestamp), 0).Format(time.RFC3339)}
	}

	pk := WorkflowInstanceProcessorStatusKey(workflowInstanceUuid, processorUuid)

	_, err := r.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.TableName),
		Key:                       pk,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	})

	if err != nil {
		return fmt.Errorf("error updating workflow instance %s processor %s status to %s: %w", workflowInstanceUuid, processorUuid, event.Status, err)
	}

	return nil
}

func (r *WorkflowInstanceProcessorStatusDatabaseStore) Put(ctx context.Context, workflowInstanceUuid string, processorUuid string, event models.WorkflowInstanceStatusEvent) error {
	status := WorkflowInstanceProcessorStatus{
		WorkflowInstanceUuid: workflowInstanceUuid,
		ProcessorUuid:        processorUuid,
		Status:               event.Status,
	}

	if event.Status == models.WorkflowInstanceStatusStarted {
		status.StartedAt = time.Unix(int64(event.Timestamp), 0).Format(time.RFC3339)
	} else if models.IsEndStateWorkflowInstanceStatus(event.Status) {
		status.CompletedAt = time.Unix(int64(event.Timestamp), 0).Format(time.RFC3339)
	}

	item, err := attributevalue.MarshalMap(status)

	pk := status.GetKey()
	for k, v := range pk {
		item[k] = v
	}

	if err != nil {
		return err
	}

	_, err = r.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.TableName),
		Item:      item,
	})

	if err != nil {
		return fmt.Errorf("error writing workflow instance %s processor %s status: %w", workflowInstanceUuid, processorUuid, err)
	}

	return nil
}
