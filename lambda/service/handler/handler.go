package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"log"

	"github.com/aws/aws-lambda-go/events"
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

	// get application data - API endpoint better than DB query
	// /applications/<applicationId>
	store := store.NewStore()
	application, _ := store.GetById(integration.ApplicationID)

	// trigger integration
	t := trigger.NewApplicationTrigger(application, integration.TriggerPayload)
	err = t.Run()
	if err != nil {
		log.Println(err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       "IntegrationServiceHandler",
		}, errors.New("error running trigger")
	}

	response := events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body: fmt.Sprintf("hello your sessionToken is %s, and your datasetId is %s",
			integration.SessionToken, integration.DatasetID),
	}
	return response, nil
}
