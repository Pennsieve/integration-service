package store_dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

type WorkflowInstanceStatusDBStore interface {
	GetAll(context.Context, string) ([]WorkflowInstanceStatus, error)
}

type WorkflowInstanceStatusDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewWorkflowInstanceStatusDatabaseStore(db *dynamodb.Client, tableName string) WorkflowInstanceStatusDBStore {
	return &WorkflowInstanceStatusDatabaseStore{db, tableName}
}

func (r *WorkflowInstanceStatusDatabaseStore) GetAll(ctx context.Context, uuid string) ([]WorkflowInstanceStatus, error) {
	result, err := r.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.TableName),
		KeyConditionExpression: aws.String("uuid = :uuid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uuid": &types.AttributeValueMemberS{Value: uuid},
		},
	})

	if err != nil {
		return []WorkflowInstanceStatus{}, fmt.Errorf("error fetching workflow instance statuses: %w", err)
	}

	var items []WorkflowInstanceStatus
	err = attributevalue.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		return []WorkflowInstanceStatus{}, fmt.Errorf("error unmarshalling workflow instance statuses: %w", err)
	}

    return items, nil
}
