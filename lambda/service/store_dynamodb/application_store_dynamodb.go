package store_dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type ApplicationDBStore interface {
	GetBySourceUrl(ctx context.Context, sourceUrl string, workspaceId string) ([]Application, error)
	Insert(ctx context.Context, application Application) error // convenience method for tests
}

type ApplicationDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewApplicationDatabaseStore(db *dynamodb.Client, tableName string) ApplicationDBStore {
	return &ApplicationDatabaseStore{db, tableName}
}

func (r *ApplicationDatabaseStore) GetBySourceUrl(ctx context.Context, sourceUrl string, organizationId string) ([]Application, error) {
	applications := []Application{}

	var c expression.ConditionBuilder
	c = expression.Name("organizationId").Equal((expression.Value(organizationId)))
	c = c.And(expression.Name("sourceUrl").Equal((expression.Value(sourceUrl))))

	expr, err := expression.NewBuilder().WithFilter(c).Build()
	if err != nil {
		return applications, fmt.Errorf("error building expression: %w", err)
	}

	response, err := r.DB.Scan(ctx, &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(r.TableName),
	})
	if err != nil {
		return applications, fmt.Errorf("error getting applications: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(response.Items, &applications)
	if err != nil {
		return applications, fmt.Errorf("error unmarshaling applications: %w", err)
	}

	return applications, nil
}

// convenience method for tests
func (r *ApplicationDatabaseStore) Insert(ctx context.Context, application Application) error {
	av, err := attributevalue.MarshalMap(application)
	if err != nil {
		return fmt.Errorf("error marshaling application: %w", err)
	}

	_, err = r.DB.PutItem(ctx, &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(r.TableName),
	})
	if err != nil {
		return fmt.Errorf("error inserting application: %w", err)
	}

	return nil
}
