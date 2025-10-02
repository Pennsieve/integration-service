package store_dynamodb

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestInsertGetApplicationBySourceUrl(t *testing.T) {
	tableName := "applications"
	dynamoDBClient := getClient()

	// create table
	_, err := CreateApplicationsTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	dynamo_store := NewApplicationDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	applicationUuid := id.String()
	organizationId := "someOrganizationId"

	application := Application{
		Uuid:           applicationUuid,
		Name:           "test-app",
		SourceUrl:      "git://some-url.com",
		OrganizationId: organizationId,
	}
	err = dynamo_store.Insert(context.Background(), application)
	if err != nil {
		t.Errorf("error inserting item into table")
	}

	applications, err := dynamo_store.GetBySourceUrl(context.Background(), application.SourceUrl, "")
	if err != nil {
		t.Errorf("error getting items in table")
	}
	assert.Equal(t, 0, len(applications))

	applications, err = dynamo_store.GetBySourceUrl(context.Background(), application.SourceUrl, organizationId)
	if err != nil {
		t.Errorf("error getting items in table")
	}
	assert.Equal(t, 1, len(applications))
	assert.Equal(t, "test-app", applications[0].Name)
	assert.Equal(t, "git://some-url.com", applications[0].SourceUrl)

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}

}

func CreateApplicationsTable(dynamoDBClient *dynamodb.Client, tableName string) (*types.TableDescription, error) {
	var tableDesc *types.TableDescription
	table, err := dynamoDBClient.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{{
			AttributeName: aws.String("uuid"),
			AttributeType: types.ScalarAttributeTypeS,
		}},
		KeySchema: []types.KeySchemaElement{{
			AttributeName: aws.String("uuid"),
			KeyType:       types.KeyTypeHash,
		}},
		TableName:   aws.String(tableName),
		BillingMode: "PAY_PER_REQUEST",
	})
	if err != nil {
		log.Printf("couldn't create table %v. Here's why: %v\n", tableName, err)
	} else {
		waiter := dynamodb.NewTableExistsWaiter(dynamoDBClient)
		err = waiter.Wait(context.TODO(), &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName)}, 5*time.Minute)
		if err != nil {
			log.Printf("wait for table exists failed. Here's why: %v\n", err)
		}
		tableDesc = table.TableDescription
	}
	return tableDesc, err
}
