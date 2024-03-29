package mocks

import (
	"bytes"
	"context"

	"github.com/pennsieve/integration-service/service/authorization"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

type MockClient struct{}

func (c *MockClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
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

func (r *MockDynamoDBStore) Insert(ctx context.Context, integration store_dynamodb.Integration) error {

	return nil
}

func (r *MockDynamoDBStore) GetById(ctx context.Context, integrationId string) (store_dynamodb.Integration, error) {

	return store_dynamodb.Integration{}, nil
}

func NewMockDynamoDBStore() store_dynamodb.DynamoDBStore {
	return &MockDynamoDBStore{}
}
