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
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/compute_trigger"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

func PostWorkflowsHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "PostWorkflowsHandler"
	var integration models.Integration
	if err := json.Unmarshal([]byte(request.Body), &integration); err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrUnmarshaling
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrConfig
	}
	dynamoDBClient := dynamodb.NewFromConfig(cfg)

	integrationsTable := os.Getenv("INTEGRATIONS_TABLE")
	dynamo_store := store_dynamodb.NewIntegrationDatabaseStore(dynamoDBClient, integrationsTable)

	// create compute node trigger
	httpClient := clients.NewComputeRestClient(&http.Client{}, integration.ComputeNode.ComputeNodeGatewayUrl)
	computeTrigger := compute_trigger.NewComputeTrigger(httpClient, integration, dynamo_store)
	// run
	if err := computeTrigger.Run(ctx); err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrRunningTrigger
	}

	response := events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body:       handlerName,
	}
	return response, nil
}
