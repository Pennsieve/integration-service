package handler

import (
	"context"
	"encoding/json"
	"errors"
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
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerName,
		}, ErrUnmarshaling
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrConfig),
		}, nil
	}
	dynamoDBClient := dynamodb.NewFromConfig(cfg)

	workflowInstanceTable := os.Getenv("INTEGRATIONS_TABLE")
	workflowInstanceStore := store_dynamodb.NewWorkflowInstanceDatabaseStore(dynamoDBClient, workflowInstanceTable)

	workflowInstance, err := workflowInstanceStore.GetById(ctx, uuid)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	workflowInstanceStatusTable := os.Getenv("WORKFLOW_INSTANCE_STATUS_TABLE")
	workflowInstanceStatusStore := store_dynamodb.NewWorkflowInstanceStatusDatabaseStore(dynamoDBClient, workflowInstanceStatusTable)

	err = workflowInstanceStatusStore.Put(ctx, workflowInstance.Uuid, requestBody)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, errors.New("failed to record workflow instance event")),
		}, nil
	}

	response := struct {
		Message string `json:"message"`
	}{
		Message: fmt.Sprintf("worklow instance %s and processor %s status updated", workflowInstance.Uuid),
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrMarshaling),
		}, err
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       string(jsonResponse),
	}, nil
}
