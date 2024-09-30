package handler

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/pennsieve/integration-service/service/authorization"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {

	if lc, ok := lambdacontext.FromContext(ctx); ok {
		log.Println("awsRequestID", lc.AwsRequestID)
	}

	applicationAuthorizer := authorization.NewApplicationAuthorizer(request)
	router := NewLambdaRouter(applicationAuthorizer)
	// register routes based on their supported methods
	router.POST("/integrations", PostIntegrationsHandler)   // deprecated
	router.GET("/integrations", GetIntegrationsHandler)     // deprecated
	router.GET("/integrations/{id}", GetIntegrationHandler) // deprecated
	router.PUT("/integrations", PutIntegrationsHandler)     // deprecated

	router.POST("/workflows", PostWorkflowsHandler)
	router.POST("/workflows/instances", PostWorkflowInstancesHandler)
	router.GET("/workflows/instances", GetWorkflowInstancesHandler)
	router.GET("/workflows/instances/{id}", GetWorkflowInstanceHandler)
	router.PUT("/workflows/instances", PutWorkflowInstancesHandler)
	return router.Start(ctx, request)
}
