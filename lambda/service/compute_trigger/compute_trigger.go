package compute_trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/mappers"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
	"github.com/pennsieve/integration-service/service/utils"
)

type Trigger interface {
	Run(ctx context.Context) error
}

type ComputeTrigger struct {
	Client                               clients.Client
	Integration                          models.WorkflowInstance
	Store                                store_dynamodb.DynamoDBStore
	WorkflowInstanceProcessorStatusStore store_dynamodb.WorkflowInstanceProcessorStatusDBStore
	OrganizationId                       string
	WorkflowStore                        store_dynamodb.WorkflowDBStore
	ApplicationStore                     store_dynamodb.ApplicationDBStore
}

func NewComputeTrigger(
	client clients.Client,
	integration models.WorkflowInstance,
	workflowInstanceStore store_dynamodb.DynamoDBStore,
	workflowInstanceProcessorStatusStore store_dynamodb.WorkflowInstanceProcessorStatusDBStore,
	organizationId string,
	workflowStore store_dynamodb.WorkflowDBStore,
	applicationStore store_dynamodb.ApplicationDBStore,
) Trigger {
	return &ComputeTrigger{client, integration, workflowInstanceStore, workflowInstanceProcessorStatusStore, organizationId, workflowStore, applicationStore}
}

// runs trigger
func (t *ComputeTrigger) Run(ctx context.Context) error {
	id := uuid.New()
	integrationId := id.String()
	now := time.Now().UTC()

	organizationId := t.OrganizationId
	var err error
	dbWorkflow, err := t.WorkflowStore.GetById(ctx, t.Integration.WorkflowUuid)
	if err != nil {
		return err
	}

	// persist to dynamodb
	store_integration := store_dynamodb.WorkflowInstance{
		Uuid:                  integrationId,
		Name:                  utils.RunName(t.Integration.Name, now),
		ComputeNodeUuid:       t.Integration.ComputeNode.ComputeNodeUuid,
		ComputeNodeGatewayUrl: t.Integration.ComputeNode.ComputeNodeGatewayUrl,
		DatasetNodeId:         t.Integration.DatasetNodeID,
		PackageIds:            t.Integration.PackageIDs,
		Workflow:              t.Integration.Workflow,
		WorkflowUuid:          t.Integration.WorkflowUuid,
		ExecutionOrder:        dbWorkflow.ExecutionOrder,
		InvocationParams:      t.Integration.InvocationParams,
		Params:                t.Integration.Params,
		OrganizationId:        organizationId,
		Status:                models.WorkflowInstanceStatusNotStarted,
	}

	var workflow []models.WorkflowProcessor

	if t.Integration.WorkflowUuid == "" {
		workflow, err = mappers.ExtractWorkflow(t.Integration.Workflow)
		if err != nil {
			return err
		}
	} else {
		workflow, err = mappers.BuildWorkflow(ctx,
			t.Integration.WorkflowUuid,
			t.WorkflowStore,
			t.ApplicationStore)
		if err != nil {
			return err
		}
	}

	if len(workflow) == 0 {
		return errors.New("cannot trigger compute for workflow instance with empty workflow")
	}

	err = t.Store.Insert(ctx, store_integration)
	if err != nil {
		return err
	}

	// store initial status for workflow instance processors
	for _, p := range workflow {
		err = t.WorkflowInstanceProcessorStatusStore.Put(ctx, integrationId, p.Uuid, models.WorkflowInstanceStatusEvent{
			Status: models.WorkflowInstanceStatusNotStarted,
		})

		if err != nil {
			return err
		}

	}

	computePayload := ComputePayload{
		IntegrationId: integrationId,
	}
	b, err := json.Marshal(computePayload)
	if err != nil {
		return err
	}

	var resp []byte
	var respError error
	authenticationMode := os.Getenv("COMPUTE_GATEWAY_AUTHENTICATION_TYPE")
	if authenticationMode == "IAM" {
		resp, respError = t.Client.Execute(ctx, *bytes.NewBuffer(b))
	} else {
		resp, respError = t.Client.ExecuteLegacy(ctx, *bytes.NewBuffer(b))
	}

	// handle responses:
	// currently we expect a 2xx response and no errors?
	if respError != nil {
		log.Println(respError.Error())
		return err
	}
	log.Println(string(resp))

	return nil
}
