package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
)

func PostWorkflowsHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "PostWorkflowsHandler"
	var workflow models.Workflow
	if err := json.Unmarshal([]byte(request.Body), &workflow); err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrUnmarshaling),
		}, nil
	}

	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	_ = claims.OrgClaim.NodeId

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrConfig),
		}, nil
	}
	_ = dynamodb.NewFromConfig(cfg)

	_ = os.Getenv("WORKFLOWS_TABLE")

	m, err := json.Marshal(models.IntegrationResponse{
		Message: "Workflow created",
	})
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrMarshaling),
		}, nil
	}

	response := events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       string(m),
	}
	return response, nil
}
