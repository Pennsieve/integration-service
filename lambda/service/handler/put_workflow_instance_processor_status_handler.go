package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/mappers"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func PutWorkflowInstanceProcessorStatusHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "PutWorkflowInstanceProcessorStatusHandler"
	uuid := request.PathParameters["id"]
	processorUuid := request.PathParameters["processorId"]

	var requestBody models.WorkflowInstanceStatusEvent
	if err := json.Unmarshal([]byte(request.Body), &requestBody); err != nil {
		return APIErrorResponse(
			handlerName,
			http.StatusInternalServerError,
			ErrUnmarshaling.Error(),
			err,
		), nil
	}

	if !models.IsValidWorkflowInstanceStatus(requestBody.Status) {
		err := fmt.Errorf("invalid workflow instance status: %s", requestBody.Status)
		return APIErrorResponse(
			handlerName,
			http.StatusBadRequest,
			err.Error(),
			err,
		), nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return APIErrorResponse(
			handlerName,
			http.StatusInternalServerError,
			ErrConfig.Error(),
			err,
		), nil
	}
	dynamoDBClient := dynamodb.NewFromConfig(cfg)

	workflowInstanceTable := os.Getenv("INTEGRATIONS_TABLE")
	workflowInstanceStore := store_dynamodb.NewWorkflowInstanceDatabaseStore(dynamoDBClient, workflowInstanceTable)

	workflowInstance, err := workflowInstanceStore.GetById(ctx, uuid)
	if err != nil {
		return APIErrorResponse(
			handlerName,
			http.StatusNotFound,
			fmt.Sprintf("workflow instance %s not found", uuid),
			err,
		), nil
	}

	workflow, err := mappers.ExtractWorkflow(workflowInstance.Workflow)
	if err != nil {
		return APIErrorResponse(
			handlerName,
			http.StatusInternalServerError,
			fmt.Sprintf("invalid workflow definition found in workflow instance: %s", workflowInstance.Uuid),
			err,
		), nil
	}

	validProcessorId := func() bool {
		for _, p := range workflow {
			if p.Uuid == processorUuid {
				return true
			}
		}
		return false
	}()

	if !validProcessorId {
		return APIErrorResponse(
			handlerName,
			http.StatusBadRequest,
			fmt.Sprintf("invalid processor %s for workflow instance %s", processorUuid, workflowInstance.Uuid),
			nil,
		), nil
	}

	workflowInstanceProcessorStatusTable := os.Getenv("WORKFLOW_INSTANCE_PROCESSOR_STATUS_TABLE")
	workflowInstanceProcessorStatusStore := store_dynamodb.NewWorkflowInstanceProcessorStatusDatabaseStore(dynamoDBClient, workflowInstanceProcessorStatusTable)

	err = workflowInstanceProcessorStatusStore.SetStatus(ctx, workflowInstance.Uuid, processorUuid, requestBody)
	if err != nil {
		return APIErrorResponse(
			handlerName,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to set %s status for workflow instance %s and processor %s", requestBody.Status, workflowInstance.Uuid, processorUuid),
			err,
		), nil
	}

	// HACK for HACKATHON: if a processor failed, set the overall workflow instance status to failed
	// This should be done separately by the wokflow manager but there is no failure handling for workflow instances in the workflow manager at the time this is written
	// All of this should be re-evaluated with the workflow instance is refactored for named workflows
	if requestBody.Status == models.WorkflowInstanceStatusFailed {
		err = workflowInstanceStore.SetStatus(ctx, workflowInstance.Uuid, requestBody)
		if err != nil {
			return APIErrorResponse(
				handlerName,
				http.StatusInternalServerError,
				fmt.Sprintf("failed to set %s status for workflow instance %s", requestBody.Status, workflowInstance.Uuid),
				err,
			), nil
		}
	}

	response := models.IntegrationResponse{
		Message: fmt.Sprintf("worklow instance %s processor %s status updated to %s", workflowInstance.Uuid, processorUuid, requestBody.Status),
	}

	return APIJsonResponse(handlerName, http.StatusOK, response), nil
}
