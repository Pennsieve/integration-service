package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	processorUuid := request.PathParameters["processorUuid"]

	var requestBody models.WorkflowInstanceStatusEvent
	if err := json.Unmarshal([]byte(request.Body), &requestBody); err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerName,
		}, ErrUnmarshaling
	}

	if !models.IsValidWorkflowInstanceStatus(requestBody.Status) {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusBadRequest,
			Body:       handlerError(handlerName, fmt.Errorf("invalid workflow instance status: %s", requestBody.Status)),
		}, nil
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Print(err)
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
		log.Print(err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	workflow, err := mappers.ExtractWorkflow(workflowInstance.Workflow)
	if err != nil {
		log.Print(err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, fmt.Errorf("invalid workflow definition found in workflow instance: %s", workflowInstance.Uuid)),
		}, nil
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
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	workflowInstanceProcessorStatusTable := os.Getenv("WORKFLOW_INSTANCE_PROCESSOR_STATUS_TABLE")
	workflowInstanceProcessorStatusStore := store_dynamodb.NewWorkflowInstanceProcessorStatusDatabaseStore(dynamoDBClient, workflowInstanceProcessorStatusTable)

	err = workflowInstanceProcessorStatusStore.SetStatus(ctx, workflowInstance.Uuid, processorUuid, requestBody)
	if err != nil {
		log.Print(err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, errors.New("failed to record workflow instance processor status")),
		}, nil
	}

	// HACK for HACKATHON: if a processor failed, set the overall workflow instance status to failed
	// This should be done separately by the wokflow manager but there is no failure handling for workflow instances in the workflow manager at the time this is written
	// All of this should be re-evaluated with the workflow instance is refactored for named workflows
	if requestBody.Status == models.WorkflowInstanceStatusFailed {
		err = workflowInstanceStore.SetStatus(ctx, workflowInstance.Uuid, requestBody)
		if err != nil {
			log.Print(err)
			return events.APIGatewayV2HTTPResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       handlerError(handlerName, ErrDynamoDB),
			}, nil
		}
	}

	response := struct {
		Message string `json:"message"`
	}{
		Message: fmt.Sprintf("worklow instance %s processor %s status updated", workflowInstance.Uuid, processorUuid),
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		log.Print(err)
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
