package compute_trigger_test

import (
	"context"
	"testing"

	"github.com/pennsieve/integration-service/service/compute_trigger"
	"github.com/pennsieve/integration-service/service/mocks"
	"github.com/pennsieve/integration-service/service/models"
)

func TestRun(t *testing.T) {
	integration := models.WorkflowInstance{}
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
