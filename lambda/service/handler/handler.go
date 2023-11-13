package handler

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/pennsieve/integration-service/service/authorization"
)

func IntegrationServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {

	if lc, ok := lambdacontext.FromContext(ctx); ok {
		log.Println("awsRequestID", lc.AwsRequestID)
	}

	resp, err := http.Get("https://api.datacite.org/dois/10.26275/eefp-azay")
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	log.Println(string(body))

	applicationAuthorizer := authorization.NewApplicationAuthorizer(request)
	router := NewLambdaRouter(applicationAuthorizer)
	// register routes based on their supported methods
	router.POST("/integrations", PostIntegrationsHandler)
	router.GET("/integrations", GetIntegrationsHandler)
	return router.Start(ctx, request)
}
