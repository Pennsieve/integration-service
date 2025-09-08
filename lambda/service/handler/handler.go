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
	router.GET("/integrations", GetIntegrationsHandler)     // deprecated
	router.GET("/integrations/{id}", GetIntegrationHandler) // deprecated
	router.PUT("/integrations", PutIntegrationsHandler)     // deprecated

	router.POST("/workflows", PostWorkflowsHandler)
	router.GET("/workflows", GetWorkflowsHandler)
	router.POST("/workflows/instances", PostWorkflowInstancesHandler)
	router.GET("/workflows/instances", GetWorkflowInstancesHandler)
	router.GET("/workflows/instances/{id}", GetWorkflowInstanceHandler)
	router.PUT("/workflows/instances", PutWorkflowInstancesHandler) // deprecated

	router.GET("/workflows/instances/{id}/logs", GetWorkflowInstanceLogsHandler)

	router.PUT("/workflows/instances/{id}/status", PutWorkflowInstanceStatusHandler)
	router.PUT("/workflows/instances/{id}/processor/{processorId}/status", PutWorkflowInstanceProcessorStatusHandler)
	router.GET("/workflows/instances/{id}/status", GetWorkflowInstanceStatusHandler)

	return router.Start(ctx, request)
}
