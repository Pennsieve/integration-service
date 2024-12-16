package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/log_retriever"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func GetWorkflowInstanceLogsHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "GetWorkflowInstanceLogsHandler"
	uuid := request.PathParameters["id"]
	queryParams := request.QueryStringParameters

	applicationUuid, found := queryParams["applicationUuid"]
	if !found {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       handlerError(handlerName, ErrMissingParameter),
		}, nil
	}

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

	if integration.ComputeNodeGatewayUrl == "" {
		log.Println("compute node URL required")
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       handlerError(handlerName, ErrRunningLogRetriever),
		}, nil

	}

	httpClient := clients.NewComputeRestClient(&http.Client{}, fmt.Sprintf("%s/logs", integration.ComputeNodeGatewayUrl))
	logRetriever := log_retriever.NewLogRetriever(httpClient, uuid, applicationUuid)
	// retrieve logs
	resp, err := logRetriever.Run(ctx)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrRunningLogRetriever),
		}, nil
	}

	response := events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       string(resp),
	}
	return response, nil
}
