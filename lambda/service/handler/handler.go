package handler

import (
	"context"

	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		log.Println("Processing awsRequestID:", lc.AwsRequestID)
	}

	router := NewLambdaRouter()
	router.POST("/integrations", PostIntegrationsHandler)
	return router.Start(ctx, request)
}
