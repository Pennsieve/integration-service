package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/log_retriever"
	"github.com/pennsieve/integration-service/service/mappers"
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

	workflowInstancesTable := os.Getenv("INTEGRATIONS_TABLE")
	dynamoStore := store_dynamodb.NewWorkflowInstanceDatabaseStore(dynamoDBClient, workflowInstancesTable)

	workflowInstance, err := dynamoStore.GetById(ctx, uuid)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNotFound,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	if workflowInstance.ComputeNodeGatewayUrl == "" {
		log.Println("compute node URL required")
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       handlerError(handlerName, ErrRunningLogRetriever),
		}, nil

	}

	// create compute node trigger
	httpClient := clients.NewComputeRestClient(&http.Client{}, fmt.Sprintf("%slogs", workflowInstance.ComputeNodeGatewayUrl),
		os.Getenv("REGION"),
		cfg,
		workflowInstance.AccountId)
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

	mappedResponse, err := mappers.ServiceResponseToAuxiliaryResponse(resp)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrRunningLogRetriever),
		}, nil
	}

	logs, err := json.Marshal(mappedResponse)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrMarshaling),
		}, nil
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       string(logs),
	}, nil
}
