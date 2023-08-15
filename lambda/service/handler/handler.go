package handler

import (
	"context"
	"os"

	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/pennsieve/integration-service/service/authorization"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "IntegrationServiceHandler"
	programLevel := new(slog.LevelVar)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel}))
	slog.SetDefault(logger)

	if lc, ok := lambdacontext.FromContext(ctx); ok {
		logger.With("awsRequestID", lc.AwsRequestID)
	}

	db, err := pgQueries.ConnectRDS()
	if err != nil {
		logger.Error("%v", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrDatabaseConnection
	}
	defer db.Close()

	authorizationHelper := authorization.NewClaimsAuthorizationHelper(request, db)
	if authorizationHelper.IsAuthorized() {
		router := NewLambdaRouter()
		// register routes based on their supported methods
		router.POST("/integrations", PostIntegrationsHandler)
		return router.Start(ctx, request)
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 409,
		Body:       handlerName,
	}, ErrUnauthorized
}
