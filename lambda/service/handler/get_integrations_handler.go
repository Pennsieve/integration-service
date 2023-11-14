package handler

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
	"github.com/pennsieve/integration-service/service/utils"
)

func GetIntegrationsHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "GetIntegrationsHandler"
	uuid := utils.ExtractParam(request.RawPath)

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
	integration, err := dynamo_store.GetById(ctx, uuid)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 404,
			Body:       handlerName,
		}, ErrNoRecordsFound
	}

	m, err := json.Marshal(models.Integration{
		Uuid:          integration.Uuid,
		ApplicationID: integration.ApplicationId,
		DatasetNodeID: integration.DatasetNodeId,
		PackageIDs:    integration.PackageIds,
	})
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrMarshaling
	}
	response := events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body:       string(m),
	}
	return response, nil
}
