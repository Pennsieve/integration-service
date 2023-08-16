package mocks

import (
	"bytes"
	"context"

	"github.com/pennsieve/integration-service/service/authorization"
	"github.com/pennsieve/integration-service/service/clients"
)

type MockClient struct{}

func (c *MockClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	return nil, nil
}

func NewMockClient() clients.Client {
	return &MockClient{}
}

type MockApplicationAuthorizer struct{}

func (c *MockApplicationAuthorizer) IsAuthorized() bool {
	return true
}

func NewMockApplicationAuthorizer() authorization.ServiceAuthorizer {
	return &MockApplicationAuthorizer{}
}
