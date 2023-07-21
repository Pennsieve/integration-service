package trigger_test

import (
	"context"
	"testing"

	"github.com/pennsieve/integration-service/service/mocks"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/trigger"
)

func TestValidate(t *testing.T) {
	application := models.Application{
		ID:         1,
		Name:       "mockApplication",
		URL:        "http://localhost:8081/mock",
		IsActive:   false,
		IsInternal: false,
	}
	triggerPayload := models.TriggerPayload{
		PackageIDs: []int64{1, 2, 3},
	}

	mockClient := mocks.NewMockClient()
	applicationTrigger := trigger.NewApplicationTrigger(mockClient, application, triggerPayload)
	err := applicationTrigger.Validate()
	expectedError := "application should be active"
	if err.Error() != expectedError {
		t.Errorf("expected: %s, got %s", expectedError, err)
	}
}

func TestRun(t *testing.T) {
	application := models.Application{
		ID:         1,
		Name:       "mockApplication",
		URL:        "http://localhost:8081/mock",
		IsActive:   true,
		IsInternal: false,
	}
	triggerPayload := models.TriggerPayload{
		PackageIDs: []int64{1, 2, 3},
	}

	mockClient := mocks.NewMockClient()
	applicationTrigger := trigger.NewApplicationTrigger(mockClient, application, triggerPayload)
	ctx := context.Background()
	err := applicationTrigger.Run(ctx)
	if err != nil {
		t.Error(err)
	}
}
