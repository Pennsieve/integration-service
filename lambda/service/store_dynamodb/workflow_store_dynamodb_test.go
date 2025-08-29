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
	"github.com/pennsieve/integration-service/service/models"
	"github.com/stretchr/testify/assert"
)

func TestInsertGetWorkflows(t *testing.T) {
	tableName := "workflows"
	dynamoDBClient := getClient()

	// create table
	_, err := CreateWorkflowsTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	dynamo_store := NewWorkflowDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	workflowUuid := id.String()
	processors := []models.Processor{
		{
			SourceUrl: "appUrl1",
			DependsOn: []models.ProcessorDependency{},
		},
		{
			SourceUrl: "appUrl2",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl1"},
			},
		},
		{
			SourceUrl: "appUrl3",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl2"},
			},
		},
	}
	organizationId := "someOrganizationId"

	workflow := Workflow{
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

	workflows, err := dynamo_store.Get(context.Background(), organizationId)
	if err != nil {
		t.Errorf("error getting items in table")
	}

	assert.Equal(t, 1, len(workflows))
	assert.Equal(t, "cytof-pipeline", workflows[0].Name)

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}

}

func TestInsertGetByIdWorkflows(t *testing.T) {
	tableName := "workflows"
	dynamoDBClient := getClient()

	// create table
	_, err := CreateWorkflowsTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	dynamo_store := NewWorkflowDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	workflowUuid := id.String()
	processors := []models.Processor{
		{
			SourceUrl: "appUrl1",
			DependsOn: []models.ProcessorDependency{},
		},
		{
			SourceUrl: "appUrl2",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl1"},
			},
		},
		{
			SourceUrl: "appUrl3",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl2"},
			},
		},
	}
	organizationId := "someOrganizationId"

	workflow := Workflow{
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

	workflow, err = dynamo_store.GetById(context.Background(), workflowUuid)
	if err != nil {
		t.Errorf("error getting item in table")
	}
	assert.Equal(t, "cytof-pipeline", workflow.Name)

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
