package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/integration-service/service/trigger"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	body := []byte(request.Body)
	var integration models.Integration

	err := json.Unmarshal(body, &integration)
	if err != nil {
		log.Println(err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       "IntegrationServiceHandler",
		}, errors.New("error unmarshaling body")
	}

	// get application data
	store := store.NewStore()
	application, _ := store.GetById(integration.ApplicationID)

	// trigger integration
	client := clients.NewApplicationRestClient(&http.Client{}, application.URL)
	applicationTrigger := trigger.NewApplicationTrigger(client,
		application,
		integration.TriggerPayload)
	err = applicationTrigger.Run()
	if err != nil {
		log.Println(err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       "IntegrationServiceHandler",
		}, err // replace with generic error message
	}

	response := events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body: fmt.Sprintf("routeKey: %s | sessionToken: %s | datasetId: %s",
			request.RouteKey, integration.SessionToken, integration.DatasetID),
	}
	return response, nil
}
