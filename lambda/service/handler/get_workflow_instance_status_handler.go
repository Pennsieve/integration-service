package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

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

	workflowInstanceStatusTable := os.Getenv("WORKFLOW_INSTANCE_STATUS_TABLE")
	workflowInstanceStatusStore := store_dynamodb.NewWorkflowInstanceStatusDatabaseStore(dynamoDBClient, workflowInstanceStatusTable)

	workflowInstanceStatuses, err := workflowInstanceStatusStore.GetAll(ctx, uuid)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	statuses := groupStatusesByProcessor(workflowInstanceStatuses)

	response := models.WorkflowInstanceStatus{
		StatusMetadata: models.StatusMetadata{
			Uuid:        workflowInstance.Uuid,
			Status:      "NOT_STARTED",
			StartedAt:   workflowInstance.StartedAt,
			CompletedAt: workflowInstance.CompletedAt,
		},
		Processors: []models.WorkflowProcessorStatus{},
	}

	for _, status := range statuses {
		ps := status.ProcessorStatus
		if ps.Uuid == workflowInstance.Uuid {
			response.Status = ps.Status
		} else {
			response.Processors = append(response.Processors, models.WorkflowProcessorStatus{
				StatusMetadata: ps,
			})
		}
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

// Aggregates processor status events into a single current state
type groupedProcessorStatuses map[string]struct {
	ProcessorStatus models.StatusMetadata
	Latest          store_dynamodb.WorkflowInstanceStatus
}

func groupStatusesByProcessor(workflowInstanceStatuses []store_dynamodb.WorkflowInstanceStatus) groupedProcessorStatuses {
	statuses := make(groupedProcessorStatuses)

	for _, item := range workflowInstanceStatuses {
		current, exists := statuses[item.ProcessorUuid]
		ps, latest := current.ProcessorStatus, current.Latest
		if !exists {
			ps = models.StatusMetadata{
				Uuid:   item.ProcessorUuid,
				Status: item.Status,
			}
			latest = item
		}

		if item.Timestamp > latest.Timestamp {
			latest = item
			ps.Status = item.Status
		}

		switch item.Status {
		case "STARTED":
			ps.StartedAt = time.Unix(int64(item.Timestamp), 0).UTC().String()
		case "FAILED", "SUCCEEDED", "CANCELLED":
			ps.CompletedAt = time.Unix(int64(item.Timestamp), 0).UTC().String()
		}

		statuses[item.ProcessorUuid] = struct {
			ProcessorStatus models.StatusMetadata
			Latest          store_dynamodb.WorkflowInstanceStatus
		}{
			ProcessorStatus: ps,
			Latest:          latest,
		}
	}

	return statuses
}
