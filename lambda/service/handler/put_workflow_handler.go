package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
)

func PutWorkflowHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "PutWorkflowHandler"
	uuid := request.PathParameters["id"]

	var workflow models.Workflow
	if err := json.Unmarshal([]byte(request.Body), &workflow); err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       handlerName,
		}, ErrUnmarshaling
	}

	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	userNodeId := claims.UserClaim.NodeId

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrConfig),
		}, nil
	}
	dynamoDBClient := dynamodb.NewFromConfig(cfg)
	tableName := os.Getenv("WORKFLOWS_TABLE")

	dynamo_store := store_dynamodb.NewWorkflowDatabaseStore(dynamoDBClient, tableName)

	store_workflow := store_dynamodb.Workflow{
		IsActive:  workflow.IsActive,
		UpdatedBy: userNodeId,
	}
	err = dynamo_store.Update(ctx, store_workflow, uuid)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrDynamoDB),
		}, nil
	}

	m, err := json.Marshal(models.GenericResponse{
		Message: fmt.Sprintf("workflow %v updated", uuid),
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
