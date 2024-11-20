package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func GetWorkflowInstanceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "GetWorkflowInstanceHandler"
	uuid := request.PathParameters["id"]

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrConfig),
		}, nil
	}
	dynamoDBClient := dynamodb.NewFromConfig(cfg)
	integrationsTable := os.Getenv("INTEGRATIONS_TABLE")

	dynamo_store := store_dynamodb.NewWorkflowInstanceDatabaseStore(dynamoDBClient, integrationsTable)
	integration, err := dynamo_store.GetById(ctx, uuid)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	m, err := json.Marshal(models.WorkflowInstance{
		Uuid: integration.Uuid,
		ComputeNode: models.ComputeNode{
			ComputeNodeUuid: integration.ComputeNodeUuid,
		},
		DatasetNodeID: integration.DatasetNodeId,
		PackageIDs:    integration.PackageIds,
		Workflow:      integration.Workflow,
		Params:        integration.Params,
		StartedAt:     integration.StartedAt,
		CompletedAt:   integration.CompletedAt,
	})
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrMarshaling),
		}, nil
	}
	response := events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       string(m),
	}
	return response, nil
}
