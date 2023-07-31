package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/integration-service/service/trigger"
	"github.com/pennsieve/integration-service/service/utils"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "IntegrationServiceHandler"
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		log.Println("Processing awsRequestID:", lc.AwsRequestID)
	}

	switch utils.ExtractRoute(request.RouteKey) {
	case "/integrations":
		switch request.RequestContext.HTTP.Method {
		case "POST":
			var integration models.Integration
			if err := json.Unmarshal([]byte(request.Body), &integration); err != nil {
				log.Println(err)
				return events.APIGatewayV2HTTPResponse{
					StatusCode: 500,
					Body:       handlerName,
				}, ErrUnmarshaling
			}

			db, err := pgQueries.ConnectRDS()
			if err != nil {
				log.Print(err)
				return events.APIGatewayV2HTTPResponse{
					StatusCode: 500,
					Body:       handlerName,
				}, ErrDatabaseConnection
			}
			defer db.Close()

			store := store.NewApplicationDatabaseStore(db, integration.OrganizationID)
			application, err := store.GetById(ctx, integration.ApplicationID)
			if err != nil {
				log.Print(err)
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
				log.Println(err)
				return events.APIGatewayV2HTTPResponse{
					StatusCode: 409,
					Body:       handlerName,
				}, ErrApplicationValidation
			}
			// run
			if err := applicationTrigger.Run(ctx); err != nil {
				log.Println(err)
				return events.APIGatewayV2HTTPResponse{
					StatusCode: 500,
					Body:       handlerName,
				}, ErrRunningTrigger
			}
		}
	default:
		log.Println("unsupported route")
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 404,
			Body:       handlerName,
		}, ErrUnsupportedRoute

	}

	response := events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body:       handlerName,
	}
	return response, nil
}
