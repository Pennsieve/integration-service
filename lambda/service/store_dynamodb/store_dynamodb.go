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

type DynamoDBStore interface {
	Insert(context.Context, Integration) error
	GetById(context.Context, string) (Integration, error)
	Get(context.Context, string, map[string]string) ([]Integration, error)
	Update(context.Context, Integration, string) error
}

type IntegrationDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewIntegrationDatabaseStore(db *dynamodb.Client, tableName string) DynamoDBStore {
	return &IntegrationDatabaseStore{db, tableName}
}

func (r *IntegrationDatabaseStore) Insert(ctx context.Context, integration Integration) error {
	item, err := attributevalue.MarshalMap(integration)
	if err != nil {
		return err
	}
	_, err = r.DB.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(r.TableName), Item: item,
	})
	if err != nil {
		log.Printf("couldn't add integration to table. Here's why: %v\n", err)
	}
	return err
}

func (r *IntegrationDatabaseStore) GetById(ctx context.Context, integrationId string) (Integration, error) {
	integration := Integration{Uuid: integrationId}
	response, err := r.DB.GetItem(ctx, &dynamodb.GetItemInput{
		Key: integration.GetKey(), TableName: aws.String(r.TableName),
	})
	if err != nil {
		log.Printf("couldn't get info about %v. Here's why: %v\n", integrationId, err)
	} else {
		err = attributevalue.UnmarshalMap(response.Item, &integration)
		if err != nil {
			log.Printf("couldn't unmarshal response. Here's why: %v\n", err)
		}
	}

	return integration, err
}

func (r *IntegrationDatabaseStore) Get(ctx context.Context, organizationId string, params map[string]string) ([]Integration, error) {
	integrations := []Integration{}

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
		return integrations, fmt.Errorf("error building expression: %w", err)
	}

	response, err := r.DB.Scan(ctx, &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(r.TableName),
	})
	if err != nil {
		return integrations, fmt.Errorf("error getting integrations: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(response.Items, &integrations)
	if err != nil {
		return integrations, fmt.Errorf("error unmarshaling integrations: %w", err)
	}

	return integrations, nil
}

func (r *IntegrationDatabaseStore) Update(ctx context.Context, integration Integration, integrationId string) error {
	key, err := attributevalue.MarshalMap(IntegrationKey{Uuid: integrationId})
	if err != nil {
		return fmt.Errorf("error marshaling key for update: %w", err)
	}

	_, err = r.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.TableName),
		Key:       key,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c": &types.AttributeValueMemberS{Value: integration.CompletedAt},
		},
		UpdateExpression: aws.String("set completedAt = :c"),
	})
	if err != nil {
		return fmt.Errorf("error updating integration: %w", err)
	}

	return nil
}
