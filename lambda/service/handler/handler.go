package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	model "github.com/pennsieve/integration-service/service/models"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	body := []byte(request.Body)
	var integration model.Integration

	err := json.Unmarshal(body, &integration)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       "error unmarshaling body",
		}, err
	}
	response := events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body: fmt.Sprintf("hello your sessionToken is %s, and your datasetId is %s",
			integration.SessionToken, integration.DatasetID),
	}
	return response, nil
}
