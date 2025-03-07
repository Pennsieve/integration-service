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
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowInstanceProcessorStatusTable(t *testing.T) {
	tableName := "workflow-instance-status"
	dynamoDBClient := getClient()

	// create table
	_, err := CreateWorkflowInstanceProcessorStatusTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}

	dynamo_store := store_dynamodb.NewWorkflowInstanceProcessorStatusDatabaseStore(dynamoDBClient, tableName)
	workflowInstanceId := uuid.NewString()
	processorId := uuid.NewString()
	now := time.Now().Unix()

	// insert workflow processor status (should be included in GetAll)
	statusEvent := models.WorkflowInstanceStatusEvent{
		Status:    models.WorkflowInstanceStatusNotStarted,
		Timestamp: int(now),
	}
	err = dynamo_store.Put(context.Background(), workflowInstanceId, processorId, statusEvent)
	if err != nil {
		t.Errorf("error inserting items into table: %v", err)
	}

	// insert same workflow new processor status (should be included in GetAll)
	statusEvent = models.WorkflowInstanceStatusEvent{
		Status:    models.WorkflowInstanceStatusNotStarted,
		Timestamp: int(now),
	}
	err = dynamo_store.Put(context.Background(), workflowInstanceId, uuid.NewString(), statusEvent)
	if err != nil {
		t.Errorf("error inserting items into table: %v", err)
	}
	// insert new workflow same processor status (should be excluded from GetAll)
	err = dynamo_store.Put(context.Background(), uuid.NewString(), processorId, statusEvent)
	if err != nil {
		t.Errorf("error inserting items into table: %v", err)
	}

	// update the workflow processor status
	statusEvent = models.WorkflowInstanceStatusEvent{
		Status:    models.WorkflowInstanceStatusStarted,
		Timestamp: int(now) + 1,
	}
	err = dynamo_store.SetStatus(context.Background(), workflowInstanceId, processorId, statusEvent)
	if err != nil {
		t.Errorf("error inserting items into table: %v", err)
	}

	statuses, err := dynamo_store.GetAll(context.Background(), workflowInstanceId)
	if err != nil {
		t.Errorf("error getting item in table: %v", err)
	}

	assert.Len(t, statuses, 2)

	for _, status := range statuses {
		if status.ProcessorUuid == processorId {
			assert.Equal(t, status.Status, models.WorkflowInstanceStatusStarted)
		}
	}

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
}

func CreateWorkflowInstanceProcessorStatusTable(dynamoDBClient *dynamodb.Client, tableName string) (*types.TableDescription, error) {
	var tableDesc *types.TableDescription
	table, err := dynamoDBClient.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{{
			AttributeName: aws.String("workflowInstanceUuid"),
			AttributeType: types.ScalarAttributeTypeS,
		}, {
			AttributeName: aws.String("processorUuid"),
			AttributeType: types.ScalarAttributeTypeS,
		}},
		KeySchema: []types.KeySchemaElement{{
			AttributeName: aws.String("workflowInstanceUuid"),
			KeyType:       types.KeyTypeHash,
		}, {
			AttributeName: aws.String("processorUuid"),
			KeyType:       types.KeyTypeRange,
		}},
		TableName:   aws.String(tableName),
		BillingMode: "PAY_PER_REQUEST",
	})
	if err != nil {
		log.Printf("couldn't create table %v. Here's why: %v\n", tableName, err)
	} else {
		waiter := dynamodb.NewTableExistsWaiter(dynamoDBClient)
		err = waiter.Wait(context.TODO(), &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		}, 5*time.Minute)
		if err != nil {
			log.Printf("wait for table exists failed. Here's why: %v\n", err)
		}
		tableDesc = table.TableDescription
	}
	return tableDesc, err
}
