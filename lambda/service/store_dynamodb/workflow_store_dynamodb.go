package store_dynamodb

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
)

type WorkflowDBStore interface {
	Insert(context.Context, Workflow) error
	Get(context.Context, string) ([]Workflow, error)
	GetById(context.Context, string) (Workflow, error)
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

func (r *WorkflowDatabaseStore) GetById(ctx context.Context, instanceId string) (Workflow, error) {
	workflow := Workflow{Uuid: instanceId}
	response, err := r.DB.GetItem(ctx, &dynamodb.GetItemInput{
		Key: workflow.GetKey(), TableName: aws.String(r.TableName),
	})
	if err != nil {
		log.Printf("couldn't get info about %v. Here's why: %v\n", instanceId, err)
	} else {
		err = attributevalue.UnmarshalMap(response.Item, &workflow)
		if err != nil {
			log.Printf("couldn't unmarshal response. Here's why: %v\n", err)
		}
	}

	return workflow, err
}
