package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/mappers"
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

	// status request UUID should either be the workflow instance ID or one of its processors' IDs
	validProcessorID := false
	if requestBody.Uuid == workflowInstance.Uuid {
		validProcessorID = true
	} else {
		for _, p := range workflow {
			if p.Uuid == requestBody.Uuid {
				validProcessorID = true
				break
			}
		}
	}
	if !validProcessorID {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	workflowInstanceStatusTable := os.Getenv("WORKFLOW_INSTANCE_STATUS_TABLE")
	workflowInstanceStatusStore := store_dynamodb.NewWorkflowInstanceStatusDatabaseStore(dynamoDBClient, workflowInstanceStatusTable)

	err = workflowInstanceStatusStore.Put(ctx, workflowInstance.Uuid, requestBody)
	if err != nil {
		log.Print(err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, errors.New("failed to record workflow instance status event")),
		}, nil
	}

	// HACK for HACKATHON: if a processor failed, set the overall workflow instance status to failed
	// ALSO set the CompletedAt on the workflow instance
	if requestBody.Uuid != workflowInstance.Uuid && requestBody.Status == models.WorkflowInstanceStatusFailed {
		err = workflowInstanceStatusStore.Put(ctx, workflowInstance.Uuid, models.WorkflowInstanceStatusEvent{
			Uuid:      workflowInstance.Uuid,
			Status:    requestBody.Status,
			Timestamp: requestBody.Timestamp,
		})
		if err != nil {
			log.Print(err)
			return events.APIGatewayV2HTTPResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       handlerError(handlerName, errors.New("failed to record workflow instance status event")),
			}, nil
		}
		updatedWorkflowInstance := store_dynamodb.WorkflowInstance{
			CompletedAt: time.Unix(int64(requestBody.Timestamp), 0).UTC().String(),
		}
		err = workflowInstanceStore.Update(ctx, updatedWorkflowInstance, workflowInstance.Uuid)
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
		Message: fmt.Sprintf("worklow instance %s and processor %s status updated", workflowInstance.Uuid, requestBody.Uuid),
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
