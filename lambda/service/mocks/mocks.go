package mocks

import (
	"bytes"
	"context"

	"github.com/pennsieve/integration-service/service/authorization"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

type MockClient struct{}

func (c *MockClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	return nil, nil
}

func (c *MockClient) Retrieve(ctx context.Context, p map[string]string) ([]byte, error) {
	return nil, nil
}

func (c *MockClient) ExecuteLegacy(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	return nil, nil
}

func (c *MockClient) RetrieveLegacy(ctx context.Context, p map[string]string) ([]byte, error) {
	return nil, nil
}

func NewMockClient() clients.Client {
	return &MockClient{}
}

type MockApplicationAuthorizer struct{}

func (c *MockApplicationAuthorizer) IsAuthorized(ctx context.Context) bool {
	return true
}

func NewMockApplicationAuthorizer() authorization.ServiceAuthorizer {
	return &MockApplicationAuthorizer{}
}

type MockDynamoDBStore struct{}

func (r *MockDynamoDBStore) Insert(ctx context.Context, integration store_dynamodb.WorkflowInstance) error {

	return nil
}

func (r *MockDynamoDBStore) GetById(ctx context.Context, integrationId string) (store_dynamodb.WorkflowInstance, error) {

	return store_dynamodb.WorkflowInstance{}, nil
}

func (r *MockDynamoDBStore) Get(ctx context.Context, organizationId string, params map[string]string) ([]store_dynamodb.WorkflowInstance, error) {

	return []store_dynamodb.WorkflowInstance{}, nil
}

func (r *MockDynamoDBStore) Update(ctx context.Context, integration store_dynamodb.WorkflowInstance, integrationId string) error {

	return nil
}

func (r *MockDynamoDBStore) SetStatus(ctx context.Context, workflowInstanceUuid string, event models.WorkflowInstanceStatusEvent) error {

	return nil
}

func NewMockDynamoDBStore() store_dynamodb.DynamoDBStore {
	return &MockDynamoDBStore{}
}

type MockDynamoDBWorkflowInstanceStatusStore struct{}

func (r *MockDynamoDBWorkflowInstanceStatusStore) Put(ctx context.Context, workflowInstanceUuid string, processorUuid string, event models.WorkflowInstanceStatusEvent) error {
	return nil
}

func (r *MockDynamoDBWorkflowInstanceStatusStore) SetStatus(ctx context.Context, workflowInstanceUuid string, processorUuid string, event models.WorkflowInstanceStatusEvent) error {
	return nil
}

func (r *MockDynamoDBWorkflowInstanceStatusStore) GetAll(ctx context.Context, uuid string) ([]store_dynamodb.WorkflowInstanceProcessorStatus, error) {
	return []store_dynamodb.WorkflowInstanceProcessorStatus{}, nil
}

func NewMockDynamoDBWorkflowInstanceStatusStore() store_dynamodb.WorkflowInstanceProcessorStatusDBStore {
	return &MockDynamoDBWorkflowInstanceStatusStore{}
}
