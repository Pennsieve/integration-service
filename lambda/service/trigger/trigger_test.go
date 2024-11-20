package trigger_test

import (
	"context"
	"testing"

	"github.com/pennsieve/integration-service/service/mocks"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/integration-service/service/trigger"
)

func TestValidate(t *testing.T) {
	application := store.Application{
		ID:         1,
		Name:       "mockApplication",
		URL:        "http://mock-application:8081/mock",
		IsDisabled: true,
	}
	integration := models.WorkflowInstance{}

	mockClient := mocks.NewMockClient()
	mockStore := mocks.NewMockDynamoDBStore()
	applicationTrigger := trigger.NewApplicationTrigger(mockClient, application, integration, mockStore)
	err := applicationTrigger.Validate()
	expectedError := "application should be active"
	if err != nil && err.Error() != expectedError {
		t.Errorf("expected: %s, got %s", expectedError, err)
	}
}

func TestRun(t *testing.T) {
	application := store.Application{
		ID:         1,
		Name:       "mockApplication",
		URL:        "http://mock-application:8081/mock",
		IsDisabled: true,
	}
	integration := models.WorkflowInstance{}

	mockClient := mocks.NewMockClient()
	mockStore := mocks.NewMockDynamoDBStore()
	applicationTrigger := trigger.NewApplicationTrigger(mockClient, application, integration, mockStore)
	ctx := context.Background()
	err := applicationTrigger.Run(ctx)
	if err != nil {
		t.Error(err)
	}
}
