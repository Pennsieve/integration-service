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
		Body: "{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetI\": \"dataset123\", \"applicationId\": 1, \"payload\": {\"packageIds\": [1,2,3]}}",
	}
	expectedStatusCode := 200

	response, err := handler.IntegrationServiceHandler(ctx, request)
	if err != nil {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}
}
