package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/dag"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
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
	organizationNodeId := claims.OrgClaim.NodeId
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
	id := uuid.New()
	workflowId := id.String()

	graph := dag.NewDAG(workflow.Processors)
	graphData := graph.GetData()
	executionOrder, err := dag.TopologicalSortLevels(graphData)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrSortingDAG),
		}, nil
	}

	store_workflow := store_dynamodb.Workflow{
		Uuid:           workflowId,
		Name:           workflow.Name,
		Description:    workflow.Description,
		Processors:     workflow.Processors,
		OrganizationId: organizationNodeId,
		Dag:            graphData,
		ExecutionOrder: executionOrder,
		CreatedAt:      time.Now().UTC().String(),
		CreatedBy:      userNodeId,
	}
	err = dynamo_store.Insert(context.Background(), store_workflow)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrMarshaling),
		}, nil
	}

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
