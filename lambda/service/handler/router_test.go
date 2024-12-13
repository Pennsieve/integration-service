package handler_test

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/handler"
	"github.com/pennsieve/integration-service/service/mocks"
)

func TestLambdaRouterPost(t *testing.T) {
	ctx := context.Background()
	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
	}
	request := events.APIGatewayV2HTTPRequest{
		RouteKey:       "POST /IncorrectIntegrationsRoute",
		Body:           "{ \"datasetId\": \"dataset123\", \"applicationId\": 1, \"packageIds\": [\"1\"]}",
		RequestContext: requestContext,
	}

	applicationAuthorizer := mocks.NewMockApplicationAuthorizer()
	router := handler.NewLambdaRouter(applicationAuthorizer)

	// POST /integrations
	router.POST("/workflows/instances", handler.PostWorkflowInstancesHandler)
	expectedStatusCode := 404
	response, _ := router.Start(ctx, request)
	if response.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}
}

func TestLambdaRouterGet(t *testing.T) {
	ctx := context.Background()
	applicationAuthorizer := mocks.NewMockApplicationAuthorizer()
	router := handler.NewLambdaRouter(applicationAuthorizer)

	// GET /integrations/1
	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "GET",
		},
	}
	request := events.APIGatewayV2HTTPRequest{
		RouteKey:       "GET /integrations/someUUID",
		Body:           "",
		RawPath:        "/integrations/someUUID",
		RequestContext: requestContext,
	}
	var GetIntegrationsHandler = func(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		response := events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Body:       "GetIntegrationsHandler",
		}
		return response, nil
	}
	expectedStatusCode := 404
	router.GET("/integrations", GetIntegrationsHandler)
	response, _ := router.Start(ctx, request)
	if response.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}
}

func TestLambdaRouter422(t *testing.T) {
	ctx := context.Background()
	applicationAuthorizer := mocks.NewMockApplicationAuthorizer()
	router := handler.NewLambdaRouter(applicationAuthorizer)

	// Unsupported path
	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "DELETE",
		},
	}
	request := events.APIGatewayV2HTTPRequest{
		RouteKey:       "DELETE /integrations/1",
		Body:           "",
		RequestContext: requestContext,
	}
	expectedStatusCode := 422
	response, _ := router.Start(ctx, request)
	if response.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %v, got %v", expectedStatusCode, response.StatusCode)
	}
}
