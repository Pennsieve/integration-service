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

	authorizationHelper := NewClaimsAuthorizationHelper(request)
	if authorizationHelper.IsAuthorized() {
		router := NewLambdaRouter()
		// register routes based on their supported methods
		router.POST("/integrations", PostIntegrationsHandler)
		return router.Start(ctx, request)
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 409,
		Body:       "IntegrationServiceHandler",
	}, ErrUnauthorized
}
