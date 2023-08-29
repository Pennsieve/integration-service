package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/integration-service/service/trigger"
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

	db, err := pgQueries.ConnectRDS()
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrDatabaseConnection
	}
	defer db.Close()

	store := store.NewApplicationDatabaseStore(db, integration.OrganizationID)
	application, err := store.GetById(ctx, integration.ApplicationID)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 409,
			Body:       handlerName,
		}, ErrNoRecordsFound
	}

	// create application trigger
	client := clients.NewApplicationRestClient(&http.Client{}, application.URL)
	applicationTrigger := trigger.NewApplicationTrigger(client, application,
		integration.TriggerPayload)
	// validate
	if applicationTrigger.Validate() != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 409,
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
