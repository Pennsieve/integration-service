package store_dynamodb

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/stretchr/testify/assert"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getClient() *dynamodb.Client {
	testDBUri := getEnv("DYNAMODB_URL", "http://localhost:8000")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy_secret", "1234")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: testDBUri}, nil
			})),
	)
	if err != nil {
		panic(err)
	}

	svc := dynamodb.NewFromConfig(cfg)
	return svc
}

func TestInsertGetById(t *testing.T) {
	tableName := "integrations"
	dynamoDBClient := getClient()

	// create table
	_, err := CreateWorkflowInstancesTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	dynamo_store := NewWorkflowInstanceDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	integrationId := id.String()
	packageIds := []string{"packageId1", "packageId2"}
	params := `{
		"target_path" : "output-folder"
	}`
	store_integration := WorkflowInstance{
		Uuid:          integrationId,
		DatasetNodeId: "xyz",
		PackageIds:    packageIds,
		Params:        params,
	}
	err = dynamo_store.Insert(context.Background(), store_integration)
	if err != nil {
		t.Errorf("error inserting item into table")
	}
	integrationItem, err := dynamo_store.GetById(context.Background(), integrationId)
	if err != nil {
		t.Errorf("error getting item in table")
	}

	assert.Equal(t, integrationId, integrationItem.Uuid)

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}

}

func TestInsertGet(t *testing.T) {
	tableName := "integrations"
	dynamoDBClient := getClient()
	organizationId := "someOrganizationId"

	// create table
	_, err := CreateWorkflowInstancesTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	dynamo_store := NewWorkflowInstanceDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	integrationId := id.String()
	packageIds := []string{"packageId1", "packageId2"}
	params := `{
		"target_path" : "output-folder"
	}`
	store_integration := WorkflowInstance{
		Uuid:            integrationId,
		ComputeNodeUuid: "someComputeNodeUuid",
		DatasetNodeId:   "someDatasetNodeId",
		PackageIds:      packageIds,
		Params:          params,
		OrganizationId:  organizationId,
		StartedAt:       time.Now().UTC().String(),
	}
	err = dynamo_store.Insert(context.Background(), store_integration)
	if err != nil {
		t.Errorf("error inserting item into table")
	}
	queryParams := make(map[string]string)
	queryParams["computeNodeUuid"] = "someComputeNodeUuid"
	queryParams["datasetNodeId"] = "someDatasetNodeId"
	integrationItems, err := dynamo_store.Get(context.Background(), organizationId, queryParams)
	if err != nil {
		t.Errorf("error getting item in table %v", err)
	}

	assert.Equal(t, 1, len(integrationItems))

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}

}

func TestNoIntegrationInstance(t *testing.T) {
	tableName := "integrations"
	dynamoDBClient := getClient()

	// create table
	_, err := CreateWorkflowInstancesTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	t.Cleanup(func() {
		// delete table
		err = DeleteTable(dynamoDBClient, tableName)
		if err != nil {
			t.Fatalf("err deleting table")
		}
	})

	store := NewWorkflowInstanceDatabaseStore(dynamoDBClient, tableName)
	integrationId := uuid.NewString()
	_, err = store.GetById(context.Background(), integrationId)
	assert.ErrorIs(t, ErrWorkflowInstanceNotFound, err)

}

func TestInsertPut(t *testing.T) {
	tableName := "integrations"
	dynamoDBClient := getClient()
	organizationId := "someOrganizationId"

	// create table
	_, err := CreateWorkflowInstancesTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}
	dynamo_store := NewWorkflowInstanceDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	integrationId := id.String()
	packageIds := []string{"packageId1", "packageId2"}
	params := `{
		"target_path" : "output-folder"
	}`
	store_integration := WorkflowInstance{
		Uuid:            integrationId,
		ComputeNodeUuid: "someComputeNodeUuid",
		DatasetNodeId:   "someDatasetNodeId",
		PackageIds:      packageIds,
		Params:          params,
		OrganizationId:  organizationId,
		StartedAt:       time.Now().UTC().String(),
	}
	err = dynamo_store.Insert(context.Background(), store_integration)
	if err != nil {
		t.Errorf("error inserting item into table %v", err)
	}

	updated_store_integration := WorkflowInstance{
		CompletedAt: time.Now().UTC().String(),
	}

	err = dynamo_store.Update(context.Background(), updated_store_integration, integrationId)
	if err != nil {
		t.Errorf("error updating item into table %v", err)
	}

	integrationItem, err := dynamo_store.GetById(context.Background(), integrationId)
	if err != nil {
		t.Errorf("error getting item in table %v", err)
	}

	assert.Equal(t, "someComputeNodeUuid", integrationItem.ComputeNodeUuid)
	assert.NotEqual(t, "", integrationItem.CompletedAt)

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	if err != nil {
		t.Fatalf("err creating table")
	}

}

func TestSetStatus(t *testing.T) {
	tableName := "integrations"
	dynamoDBClient := getClient()
	organizationId := "someOrganizationId"

	_, err := CreateWorkflowInstancesTable(dynamoDBClient, tableName)
	assert.NoError(t, err, "error while creating workflow instance table in DynamoDB")

	workflowInstanceStore := NewWorkflowInstanceDatabaseStore(dynamoDBClient, tableName)
	id := uuid.New()
	integrationId := id.String()
	packageIds := []string{"packageId1", "packageId2"}
	params := `{
		"target_path" : "output-folder"
	}`
	store_integration := WorkflowInstance{
		Uuid:            integrationId,
		ComputeNodeUuid: "someComputeNodeUuid",
		DatasetNodeId:   "someDatasetNodeId",
		PackageIds:      packageIds,
		Params:          params,
		OrganizationId:  organizationId,
	}
	err = workflowInstanceStore.Insert(context.Background(), store_integration)
	if err != nil {
		t.Errorf("error inserting item into table %v", err)
	}

	instance, err := workflowInstanceStore.GetById(context.Background(), integrationId)
	assert.NoError(t, err)
	assert.Zero(t, instance.StartedAt)
	assert.Zero(t, instance.CompletedAt)

	err = workflowInstanceStore.SetStatus(context.Background(), integrationId, models.WorkflowInstanceStatusEvent{
		Status:    models.WorkflowInstanceStatusStarted,
		Timestamp: int(time.Now().Unix()),
	})
	assert.NoError(t, err, "failed to update workflow instance status to STARTED")

	instance, err = workflowInstanceStore.GetById(context.Background(), integrationId)
	assert.NoError(t, err)

	assert.NotZero(t, instance.StartedAt)
	assert.Zero(t, instance.CompletedAt)

	err = workflowInstanceStore.SetStatus(context.Background(), integrationId, models.WorkflowInstanceStatusEvent{
		Status:    models.WorkflowInstanceStatusSucceeded,
		Timestamp: int(time.Now().Unix()),
	})
	assert.NoError(t, err, "failed to update workflow instance status to SUCCEEDED")

	instance, err = workflowInstanceStore.GetById(context.Background(), integrationId)
	assert.NoError(t, err)

	assert.NotZero(t, instance.StartedAt)
	assert.NotZero(t, instance.CompletedAt)

	// delete table
	err = DeleteTable(dynamoDBClient, tableName)
	assert.NoError(t, err, "error while deleting workflow instance table from dynamodb")
}

func CreateWorkflowInstancesTable(dynamoDBClient *dynamodb.Client, tableName string) (*types.TableDescription, error) {
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

func DeleteTable(dynamoDBClient *dynamodb.Client, tableName string) error {
	_, err := dynamoDBClient.DeleteTable(context.TODO(), &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName)})
	if err != nil {
		log.Printf("couldn't delete table %v. Here's why: %v\n", tableName, err)
	}
	return err
}
