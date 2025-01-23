package compute_trigger_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/compute_trigger"
	"github.com/pennsieve/integration-service/service/mocks"
	"github.com/pennsieve/integration-service/service/models"
)

func TestRun(t *testing.T) {
	workflow := []struct {
		Uuid string
	}{
		{Uuid: uuid.NewString()},
		{Uuid: uuid.NewString()},
	}
	integration := models.WorkflowInstance{
		Workflow: workflow,
	}
	organizationId := "someOrganizationId"

	mockClient := mocks.NewMockClient()
	mockStore := mocks.NewMockDynamoDBStore()
	mockWorkflowInstanceStatusStore := mocks.NewMockDynamoDBWorkflowInstanceStatusStore()
	computeTrigger := compute_trigger.NewComputeTrigger(mockClient, integration, mockStore, mockWorkflowInstanceStatusStore, organizationId)
	ctx := context.Background()
	err := computeTrigger.Run(ctx)
	if err != nil {
		t.Error(err)
	}
}
