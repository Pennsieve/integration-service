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
	GetBySourceUrl(ctx context.Context, sourceUrl string) ([]Application, error)
}

type ApplicationDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewApplicationDatabaseStore(db *dynamodb.Client, tableName string) ApplicationDBStore {
	return &ApplicationDatabaseStore{db, tableName}
}

func (r *ApplicationDatabaseStore) GetBySourceUrl(ctx context.Context, sourceUrl string) ([]Application, error) {
	applications := []Application{}

	c := expression.Name("sourceUrl").Equal((expression.Value(sourceUrl)))
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
