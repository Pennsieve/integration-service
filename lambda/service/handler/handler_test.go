package handler_test

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/handler"
)

func TestIntegrationServiceHandler(t *testing.T) {
	ctx := context.Background()
	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: make(map[string]interface{}),
		},
	}
	request := events.APIGatewayV2HTTPRequest{
		RouteKey:       "POST /IncorrectIntegrationsRoute",
		Body:           "{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": 1, \"payload\": {\"packageIds\": [1,2,3]}}",
		RequestContext: requestContext,
	}

	expectedStatusCode := 409
	response, _ := handler.IntegrationServiceHandler(ctx, request)
	if response.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}
}
