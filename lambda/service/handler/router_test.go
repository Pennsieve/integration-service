package handler_test

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/handler"
)

func TestLambdaRouter(t *testing.T) {
	ctx := context.Background()
	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
	}
	request := events.APIGatewayV2HTTPRequest{
		RouteKey:       "POST /IncorrectIntegrationsRoute",
		Body:           "{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": 1, \"organizationId\": 1, \"payload\": {\"packageIds\": [1,2,3]}}",
		RequestContext: requestContext,
	}

	router := handler.NewLambdaRouter()

	// POST /integrations
	router.POST("/integrations", handler.PostIntegrationsHandler)
	expectedStatusCode := 404
	response, _ := router.Start(ctx, request)
	if response.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}

	// GET /applications
	requestContext = events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "GET",
		},
	}
	request = events.APIGatewayV2HTTPRequest{
		RouteKey:       "GET /applications",
		Body:           "",
		RequestContext: requestContext,
	}
	var GetApplicationsHandler = func(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		response := events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Body:       "GetApplicationsHandler",
		}
		return response, nil
	}
	expectedStatusCode = 200
	router.GET("/applications", GetApplicationsHandler)
	response, _ = router.Start(ctx, request)
	if response.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}

	// Unsupported path
	requestContext = events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "DELETE",
		},
	}
	request = events.APIGatewayV2HTTPRequest{
		RouteKey:       "DELETE /integrations/1",
		Body:           "",
		RequestContext: requestContext,
	}
	expectedStatusCode = 409
	response, _ = router.Start(ctx, request)
	if response.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}
}
