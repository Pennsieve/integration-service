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
	"github.com/pennsieve/integration-service/service/mappers"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
)

func GetIntegrationsHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "GetIntegrationsHandler"
	queryParams := request.QueryStringParameters

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

	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	organizationId := claims.OrgClaim.NodeId

	log.Println("claims.OrgClaim.NodeId", claims.OrgClaim.NodeId)
	log.Println("claims.OrgClaim.IntId", claims.OrgClaim.IntId)

	dynamo_store := store_dynamodb.NewWorkflowInstanceDatabaseStore(dynamoDBClient, integrationsTable)
	dynamoIntegrations, err := dynamo_store.Get(ctx, organizationId, queryParams)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrNoRecordsFound),
		}, nil
	}

	m, err := json.Marshal(mappers.DynamoDBIntegrationToJsonIntegration(dynamoIntegrations))
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrDynamoDB),
		}, nil
	}
	response := events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       string(m),
	}
	return response, nil
}
