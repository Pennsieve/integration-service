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
	var workflowInstance models.WorkflowInstance
	if err := json.Unmarshal([]byte(request.Body), &workflowInstance); err != nil {
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

	workflowInstancesTable := os.Getenv("INTEGRATIONS_TABLE")
	workflowInstanceStore := store_dynamodb.NewWorkflowInstanceDatabaseStore(dynamoDBClient, workflowInstancesTable)
	workflowTable := os.Getenv("WORKFLOWS_TABLE")
	workflowStore := store_dynamodb.NewWorkflowDatabaseStore(dynamoDBClient, workflowTable)
	workflowInstanceProcessorStatusTable := os.Getenv("WORKFLOW_INSTANCE_PROCESSOR_STATUS_TABLE")
	workflow_instance_processor_status_dynamo_store := store_dynamodb.NewWorkflowInstanceProcessorStatusDatabaseStore(dynamoDBClient, workflowInstanceProcessorStatusTable)
	applicationsTable := os.Getenv("APPLICATIONS_TABLE")
	applicationsStore := store_dynamodb.NewApplicationDatabaseStore(dynamoDBClient, applicationsTable)

	computeNodesTable := os.Getenv("COMPUTE_NODES_TABLE")
	compute_nodes_store := store_dynamodb.NewNodeDatabaseStore(dynamoDBClient, computeNodesTable)
	computeNode, err := compute_nodes_store.GetById(ctx, workflowInstance.ComputeNode.ComputeNodeUuid)
	if err != nil {
		log.Println(err.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       handlerError(handlerName, ErrDynamoDB),
		}, nil
	}

	// create compute node trigger
	httpClient := clients.NewComputeRestClient(&http.Client{},
		workflowInstance.ComputeNode.ComputeNodeGatewayUrl,
		os.Getenv("REGION"),
		cfg,
		computeNode.AccountId)
	computeTrigger := compute_trigger.NewComputeTrigger(httpClient,
		workflowInstance,
		workflowInstanceStore,
		workflow_instance_processor_status_dynamo_store,
		organizationId,
		workflowStore,
		applicationsStore)
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
