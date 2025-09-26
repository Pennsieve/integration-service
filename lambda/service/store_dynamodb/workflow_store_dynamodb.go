package store_dynamodb

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

type WorkflowDBStore interface {
	Insert(context.Context, Workflow) error
	Get(context.Context, string) ([]Workflow, error)
	GetById(context.Context, string) (Workflow, error)
	Update(context.Context, Workflow, string) error
}

type WorkflowDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewWorkflowDatabaseStore(db *dynamodb.Client, tableName string) WorkflowDBStore {
	return &WorkflowDatabaseStore{db, tableName}
}

func (r *WorkflowDatabaseStore) Insert(ctx context.Context, workflow Workflow) error {
	item, err := attributevalue.MarshalMap(workflow)
	if err != nil {
		return err
	}
	_, err = r.DB.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(r.TableName), Item: item,
	})
	if err != nil {
		log.Printf("couldn't add workflow to table. Here's why: %v\n", err)
	}
	return err
}

func (r *WorkflowDatabaseStore) Get(ctx context.Context, organizationId string) ([]Workflow, error) {
	workflows := []Workflow{}

	c := expression.Name("organizationId").Equal((expression.Value(organizationId)))
	expr, err := expression.NewBuilder().WithFilter(c).Build()
	if err != nil {
		return workflows, fmt.Errorf("error building expression: %w", err)
	}

	response, err := r.DB.Scan(ctx, &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(r.TableName),
	})
	if err != nil {
		return workflows, fmt.Errorf("error getting workflows: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(response.Items, &workflows)
	if err != nil {
		return workflows, fmt.Errorf("error unmarshaling workflows: %w", err)
	}

	return workflows, nil
}

func (r *WorkflowDatabaseStore) GetById(ctx context.Context, workflowId string) (Workflow, error) {
	workflow := Workflow{Uuid: workflowId}
	response, err := r.DB.GetItem(ctx, &dynamodb.GetItemInput{
		Key: workflow.GetKey(), TableName: aws.String(r.TableName),
	})
	if err != nil {
		log.Printf("couldn't get info about %v. Here's why: %v\n", workflowId, err)
	} else {
		err = attributevalue.UnmarshalMap(response.Item, &workflow)
		if err != nil {
			log.Printf("couldn't unmarshal response. Here's why: %v\n", err)
		}
	}

	return workflow, err
}

func (r *WorkflowDatabaseStore) Update(ctx context.Context, workflow Workflow, workflowUuid string) error {
	key, err := attributevalue.MarshalMap(WorkflowInstanceKey{Uuid: workflowUuid})
	if err != nil {
		return fmt.Errorf("error marshaling key for update: %w", err)
	}

	_, err = r.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.TableName),
		Key:       key,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c": &types.AttributeValueMemberBOOL{Value: workflow.IsActive},
			":d": &types.AttributeValueMemberS{Value: workflow.UpdatedBy},
		},
		UpdateExpression: aws.String("set completedAt = :c , updatedBy = :d"),
	})
	if err != nil {
		return fmt.Errorf("error updating workflow: %w", err)
	}

	return nil
}
