package handler_test

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/handler"
)

func TestIntegrationServiceHandler(t *testing.T) {
	ctx := context.Background()
	request := events.APIGatewayV2HTTPRequest{
		Body: "{ \"sessionToken\": \"xyz12345\",  \"datasetId\": \"data123\"}",
	}
	expectedStatusCode := 200

	response, err := handler.IntegrationServiceHandler(ctx, request)
	if err != nil {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}
}
