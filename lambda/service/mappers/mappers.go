package mappers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pennsieve/integration-service/service/log_retriever"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func DynamoDBIntegrationToJsonIntegration(dynamoIntegrations []store_dynamodb.WorkflowInstance) []models.WorkflowInstance {
	integrations := []models.WorkflowInstance{}

	for _, a := range dynamoIntegrations {
		integrations = append(integrations, models.WorkflowInstance{
			Uuid: a.Uuid,
			Name: a.Name,
			ComputeNode: models.ComputeNode{
				ComputeNodeUuid:       a.ComputeNodeUuid,
				ComputeNodeGatewayUrl: a.ComputeNodeGatewayUrl,
			},
			DatasetNodeID:    a.DatasetNodeId,
			PackageIDs:       a.PackageIds,
			Workflow:         a.Workflow,
			WorkflowUuid:     a.WorkflowUuid,
			InvocationParams: a.InvocationParams,
			Params:           a.Params,
			Status:           a.Status,
			StartedAt:        a.StartedAt,
			CompletedAt:      a.CompletedAt,
		})
	}

	return integrations
}

func ServiceResponseToAuxiliaryResponse(resp []byte) (log_retriever.ProcessorLogPayload, error) {
	var m log_retriever.ProcessorLogPayload
	if err := json.Unmarshal(resp, &m); err != nil {
		return log_retriever.ProcessorLogPayload{}, err
	}

	return m, nil
}

func ExtractWorkflow(workflow interface{}) ([]models.WorkflowProcessor, error) {
	workflowBytes, err := json.Marshal(workflow)
	if err != nil {
		return nil, err
	}

	var wf []models.WorkflowProcessor
	err = json.Unmarshal(workflowBytes, &wf)
	if err != nil {
		return nil, err
	}

	return wf, nil
}

func DynamoDBWorkflowToJsonWorkflow(dynamoWorkflows []store_dynamodb.Workflow) []models.Workflow {
	workflows := []models.Workflow{}

	for _, a := range dynamoWorkflows {
		workflows = append(workflows, models.Workflow{
			Uuid:           a.Uuid,
			Name:           a.Name,
			Description:    a.Description,
			Processors:     a.Processors,
			OrganizationId: a.OrganizationId,
			Dag:            a.Dag,
			ExecutionOrder: a.ExecutionOrder,
			CreatedAt:      a.CreatedAt,
			CreatedBy:      a.CreatedBy,
		})
	}

	return workflows
}

func BuildWorkflow(ctx context.Context, uuid string, workflowStore store_dynamodb.WorkflowDBStore, applicationStore store_dynamodb.ApplicationDBStore) ([]models.WorkflowProcessor, error) {
	dbWorkflow, err := workflowStore.GetById(ctx, uuid)
	if err != nil {
		return nil, err
	}

	workflow := models.Workflow{
		ExecutionOrder: dbWorkflow.ExecutionOrder,
	}

	var wf []models.WorkflowProcessor
	for _, processor := range workflow.ExecutionOrder {
		applications, err := applicationStore.GetBySourceUrl(ctx, processor[0])
		if err != nil {
			return nil, err
		}
		if len(applications) == 0 {
			return nil, errors.New("no application found for processor source URL: " + processor[0])
		}
		wf = append(wf, models.WorkflowProcessor{
			Uuid: applications[0].Uuid,
		})
	}

	return wf, nil
}
