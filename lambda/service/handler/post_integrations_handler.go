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
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
	"github.com/pennsieve/integration-service/service/trigger"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

func PostIntegrationsHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "PostIntegrationsHandler"
	var integration models.Integration
	if err := json.Unmarshal([]byte(request.Body), &integration); err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrUnmarshaling
	}

	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	organizationId := claims.OrgClaim.IntId

	db, err := pgQueries.ConnectRDS()
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrDatabaseConnection
	}
	defer db.Close()

	store := store.NewApplicationDatabaseStore(db, organizationId)
	application, err := store.GetById(ctx, integration.ApplicationID)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 422,
			Body:       handlerName,
		}, ErrNoRecordsFound
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
	log.Println(integrationsTable)
	dynamo_store := store_dynamodb.NewIntegrationDatabaseStore(dynamoDBClient, integrationsTable)

	// create application trigger
	httpClient := clients.NewApplicationRestClient(&http.Client{}, application.URL)
	applicationTrigger := trigger.NewApplicationTrigger(httpClient, application,
		integration, dynamo_store)
	// validate
	if applicationTrigger.Validate() != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 422,
			Body:       handlerName,
		}, ErrApplicationValidation
	}
	// run
	if err := applicationTrigger.Run(ctx); err != nil {
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
