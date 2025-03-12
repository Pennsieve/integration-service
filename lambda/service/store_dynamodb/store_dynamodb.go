package store_dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/pennsieve/integration-service/service/models"
)

type DynamoDBStore interface {
	Insert(context.Context, WorkflowInstance) error
	GetById(context.Context, string) (WorkflowInstance, error)
	Get(context.Context, string, map[string]string) ([]WorkflowInstance, error)
	Update(context.Context, WorkflowInstance, string) error
	SetStatus(context.Context, string, models.WorkflowInstanceStatusEvent) error
}

type WorkflowInstanceDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewWorkflowInstanceDatabaseStore(db *dynamodb.Client, tableName string) DynamoDBStore {
	return &WorkflowInstanceDatabaseStore{db, tableName}
}

func (r *WorkflowInstanceDatabaseStore) Insert(ctx context.Context, instance WorkflowInstance) error {
	item, err := attributevalue.MarshalMap(instance)
	if err != nil {
		return err
	}
	_, err = r.DB.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(r.TableName), Item: item,
	})
	if err != nil {
		log.Printf("couldn't add instance to table. Here's why: %v\n", err)
	}
	return err
}

func (r *WorkflowInstanceDatabaseStore) GetById(ctx context.Context, instanceId string) (WorkflowInstance, error) {
	workflowInstance := WorkflowInstance{Uuid: instanceId}

	response, err := r.DB.GetItem(ctx, &dynamodb.GetItemInput{
		Key: workflowInstance.GetKey(), TableName: aws.String(r.TableName),
	})

	if err != nil {
		err = fmt.Errorf("couldn't get info about %v. Here's why: %w", instanceId, err)
	} else {
		if len(response.Item) == 0 {
			err = errors.New("workflow instance not found")
		} else {
			err = attributevalue.UnmarshalMap(response.Item, &workflowInstance)
			if err != nil {
				err = fmt.Errorf("couldn't unmarshal response. Here's why: %w", err)
			}
		}
	}

	return workflowInstance, err
}

func (r *WorkflowInstanceDatabaseStore) Get(ctx context.Context, organizationId string, params map[string]string) ([]WorkflowInstance, error) {
	workflowInstances := []WorkflowInstance{}

	var c expression.ConditionBuilder
	c = expression.Name("organizationId").Equal((expression.Value(organizationId)))

	if computeNodeUuid, found := params["computeNodeUuid"]; found {
		c = c.And(expression.Name("computeNodeUuid").Equal((expression.Value(computeNodeUuid))))
	}

	if datasetNodeId, found := params["datasetNodeId"]; found {
		c = c.And(expression.Name("datasetNodeId").Equal((expression.Value(datasetNodeId))))
	}

	expr, err := expression.NewBuilder().WithFilter(c).Build()
	if err != nil {
		return workflowInstances, fmt.Errorf("error building expression: %w", err)
	}

	response, err := r.DB.Scan(ctx, &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(r.TableName),
	})
	if err != nil {
		return workflowInstances, fmt.Errorf("error getting instances: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(response.Items, &workflowInstances)
	if err != nil {
		return workflowInstances, fmt.Errorf("error unmarshaling instances: %w", err)
	}

	return workflowInstances, nil
}

func (r *WorkflowInstanceDatabaseStore) Update(ctx context.Context, workflowInstance WorkflowInstance, instanceId string) error {
	key, err := attributevalue.MarshalMap(WorkflowInstanceKey{Uuid: instanceId})
	if err != nil {
		return fmt.Errorf("error marshaling key for update: %w", err)
	}

	_, err = r.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.TableName),
		Key:       key,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c": &types.AttributeValueMemberS{Value: workflowInstance.CompletedAt},
		},
		UpdateExpression: aws.String("set completedAt = :c"),
	})
	if err != nil {
		return fmt.Errorf("error updating instance: %w", err)
	}

	return nil
}

func (r *WorkflowInstanceDatabaseStore) SetStatus(ctx context.Context, workflowInstanceUuid string, event models.WorkflowInstanceStatusEvent) error {
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

	key, err := attributevalue.MarshalMap(WorkflowInstanceKey{Uuid: workflowInstanceUuid})
	if err != nil {
		return fmt.Errorf("error marshaling key for update: %w", err)
	}

	_, err = r.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.TableName),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return fmt.Errorf("error setting workflow instance status: %w", err)
	}

	return nil
}
