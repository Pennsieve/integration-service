package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func GetWorkflowInstanceStatusHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "GetWorkflowInstanceStatusHandler"
	uuid := request.PathParameters["id"]

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

	workflowInstanceProcessorStatusTable := os.Getenv("WORKFLOW_INSTANCE_PROCESSOR_STATUS_TABLE")
	workflowInstanceProcessorStatusStore := store_dynamodb.NewWorkflowInstanceProcessorStatusDatabaseStore(dynamoDBClient, workflowInstanceProcessorStatusTable)

	processorStatuses, err := workflowInstanceProcessorStatusStore.GetAll(ctx, uuid)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	response := models.WorkflowInstanceStatus{
		StatusMetadata: models.StatusMetadata{
			Uuid:        workflowInstance.Uuid,
			Status:      workflowInstance.Status,
			StartedAt:   workflowInstance.StartedAt,
			CompletedAt: workflowInstance.CompletedAt,
		},
		Processors: []models.WorkflowProcessorStatus{},
	}

	for _, ps := range processorStatuses {
		response.Processors = append(response.Processors, models.WorkflowProcessorStatus{
			StatusMetadata: models.StatusMetadata{
				Uuid:        ps.ProcessorUuid,
				Status:      ps.Status,
				StartedAt:   ps.StartedAt,
				CompletedAt: ps.CompletedAt,
			},
		})
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
