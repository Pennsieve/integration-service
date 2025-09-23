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
	invocationParams := map[string][]models.ProcessorParam{
		"some-git-repo1": {
			{
				Name:         "cpus",
				Value:        2,
				Type:         "integer",
				DefaultValue: 1,
				Required:     false,
			},
			{
				Name:         "env",
				Value:        "prod",
				Type:         "string",
				DefaultValue: "dev",
				Required:     false,
			},
		},
		"some-git-repo2": {
			{
				Name:         "maxFiles",
				Value:        50,
				Type:         "integer",
				DefaultValue: 100,
				Required:     false,
			},
			{
				Name:         "env",
				Value:        "prod",
				Type:         "string",
				DefaultValue: "dev",
				Required:     false,
			},
		},
	}
	workflowInstance := models.WorkflowInstance{
		Workflow:         workflow,
		InvocationParams: invocationParams,
	}
	organizationId := "someOrganizationId"

	mockClient := mocks.NewMockClient()
	mockWorkflowInstanceStore := mocks.NewMockDynamoDBStore()
	mockWorkflowInstanceStatusStore := mocks.NewMockDynamoDBWorkflowInstanceStatusStore()
	mockWorkflowStore := mocks.NewWorkflowDynamoDBStore()
	mockApplicationStore := mocks.NewApplicationDynamoDBStore()
	computeTrigger := compute_trigger.NewComputeTrigger(mockClient, workflowInstance, mockWorkflowInstanceStore, mockWorkflowInstanceStatusStore, organizationId, mockWorkflowStore, mockApplicationStore)
	ctx := context.Background()
	err := computeTrigger.Run(ctx)
	if err != nil {
		t.Error(err)
	}
}

func TestRunNoWorkflow(t *testing.T) {
	invocationParams := map[string][]models.ProcessorParam{
		"some-git-repo1": {
			{
				Name:         "cpus",
				Value:        2,
				Type:         "integer",
				DefaultValue: 1,
				Required:     false,
			},
			{
				Name:         "env",
				Value:        "prod",
				Type:         "string",
				DefaultValue: "dev",
				Required:     false,
			},
		},
		"some-git-repo2": {
			{
				Name:         "maxFiles",
				Value:        50,
				Type:         "integer",
				DefaultValue: 100,
				Required:     false,
			},
			{
				Name:         "env",
				Value:        "prod",
				Type:         "string",
				DefaultValue: "dev",
				Required:     false,
			},
		},
	}
	workflowInstance := models.WorkflowInstance{
		WorkflowUuid:     uuid.NewString(),
		InvocationParams: invocationParams,
	}
	organizationId := "someOrganizationId"

	mockClient := mocks.NewMockClient()
	mockWorkflowInstanceStore := mocks.NewMockDynamoDBStore()
	mockWorkflowInstanceStatusStore := mocks.NewMockDynamoDBWorkflowInstanceStatusStore()
	mockWorkflowStore := mocks.NewWorkflowDynamoDBStore()
	mockApplicationStore := mocks.NewApplicationDynamoDBStore()
	computeTrigger := compute_trigger.NewComputeTrigger(mockClient, workflowInstance, mockWorkflowInstanceStore, mockWorkflowInstanceStatusStore, organizationId, mockWorkflowStore, mockApplicationStore)
	ctx := context.Background()
	err := computeTrigger.Run(ctx)
	if err != nil {
		t.Error(err)
	}
}
