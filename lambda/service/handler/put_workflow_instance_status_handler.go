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
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func PutWorkflowInstanceStatusHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "PutWorkflowInstanceStatusHandler"
	uuid := request.PathParameters["id"]

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

	err = workflowInstanceStore.SetStatus(ctx, workflowInstance.Uuid, requestBody)
	if err != nil {
		return APIErrorResponse(
			handlerName,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to set %s status for workflow instance %s", requestBody.Status, workflowInstance.Uuid),
			err,
		), nil
	}

	response := models.GenericResponse{
		Message: fmt.Sprintf("worklow instance %s status updated to %s", workflowInstance.Uuid, requestBody.Status),
	}

	return APIJsonResponse(handlerName, http.StatusOK, response), nil
}
