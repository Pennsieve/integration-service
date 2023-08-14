package handler

import (
	"context"
	"os"

	"log"

	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/pennsieve/integration-service/service/authorization"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	programLevel := new(slog.LevelVar)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel}))
	slog.SetDefault(logger)

	if lc, ok := lambdacontext.FromContext(ctx); ok {
		log.Println("Processing awsRequestID:", lc.AwsRequestID)
	}

	authorizationHelper := authorization.NewClaimsAuthorizationHelper(request)
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
