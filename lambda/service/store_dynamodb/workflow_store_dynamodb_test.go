package store_dynamodb_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func TestInsertGetWorkflows(t *testing.T) {
	tableName := "workflows"
	dynamoDBClient := getClient()

	// create table
	_, err := CreateWorkflowsTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	dynamo_store := store_dynamodb.NewWorkflowDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	workflowUuid := id.String()
	processors := []string{"appUuid1", "appUuid2", "appUuid3"}
	organizationId := "someOrganizationId"

	workflow := store_dynamodb.Workflow{
		Uuid:           workflowUuid,
		Name:           "cytof-pipeline",
		Description:    "End-to-end CyTOF pipeline",
		Processors:     processors,
		OrganizationId: organizationId,
		CreatedAt:      time.Now().UTC().String(),
		CreatedBy:      "someUser",
	}
	err = dynamo_store.Insert(context.Background(), workflow)
	if err != nil {
		t.Errorf("error inserting item into table")
	}
	err = dynamo_store.Insert(context.Background(), workflow)
	if err != nil {
		t.Errorf("error inserting item in table")
	}

	workflows, err := dynamo_store.Get(context.Background(), organizationId)
	if err != nil {
		t.Errorf("error getting items in table")
	}
	if len(workflows) != 1 {
		t.Errorf("expected items in table")
	}
	if workflows[0].Name != "cytof-pipeline" {
		t.Errorf("expected item in table")
	}

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}

}

func CreateWorkflowsTable(dynamoDBClient *dynamodb.Client, tableName string) (*types.TableDescription, error) {
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
