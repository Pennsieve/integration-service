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
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/compute_trigger"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
)

func PostWorkflowInstancesHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	handlerName := "PostWorkflowInstancesHandler"
	var integration models.WorkflowInstance
	if err := json.Unmarshal([]byte(request.Body), &integration); err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrUnmarshaling),
		}, nil
	}

	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	organizationId := claims.OrgClaim.NodeId

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrConfig),
		}, nil
	}
	dynamoDBClient := dynamodb.NewFromConfig(cfg)

	integrationsTable := os.Getenv("INTEGRATIONS_TABLE")
	dynamo_store := store_dynamodb.NewWorkflowInstanceDatabaseStore(dynamoDBClient, integrationsTable)

	workflowInstanceProcessorStatusTable := os.Getenv("WORKFLOW_INSTANCE_PROCESSOR_STATUS_TABLE")
	workflow_instance_processor_status_dynamo_store := store_dynamodb.NewWorkflowInstanceProcessorStatusDatabaseStore(dynamoDBClient, workflowInstanceProcessorStatusTable)

	// create compute node trigger
	httpClient := clients.NewComputeRestClient(&http.Client{}, integration.ComputeNode.ComputeNodeGatewayUrl)
	computeTrigger := compute_trigger.NewComputeTrigger(httpClient, integration, dynamo_store, workflow_instance_processor_status_dynamo_store, organizationId)
	// run
	if err := computeTrigger.Run(ctx); err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrRunningTrigger),
		}, nil
	}

	m, err := json.Marshal(models.IntegrationResponse{
		Message: "Workflow successfully initiated",
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
