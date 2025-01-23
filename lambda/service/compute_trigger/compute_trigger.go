package compute_trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
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
	Client                      clients.Client
	Integration                 models.WorkflowInstance
	Store                       store_dynamodb.DynamoDBStore
	WorkflowInstanceStatusStore store_dynamodb.WorkflowInstanceStatusDBStore
	OrganizationId              string
}

func NewComputeTrigger(
	client clients.Client,
	integration models.WorkflowInstance,
	store store_dynamodb.DynamoDBStore,
	workflowInstanceStatusStore store_dynamodb.WorkflowInstanceStatusDBStore,
	organizationId string,
) Trigger {
	return &ComputeTrigger{client, integration, store, workflowInstanceStatusStore, organizationId}
}

// runs trigger
func (t *ComputeTrigger) Run(ctx context.Context) error {
	id := uuid.New()
	integrationId := id.String()
	startedAt := time.Now().UTC()

	// persist to dynamodb
	store_integration := store_dynamodb.WorkflowInstance{
		Uuid:                  integrationId,
		Name:                  utils.RunName(t.Integration.Name, startedAt),
		ComputeNodeUuid:       t.Integration.ComputeNode.ComputeNodeUuid,
		ComputeNodeGatewayUrl: t.Integration.ComputeNode.ComputeNodeGatewayUrl,
		DatasetNodeId:         t.Integration.DatasetNodeID,
		PackageIds:            t.Integration.PackageIDs,
		Workflow:              t.Integration.Workflow,
		Params:                t.Integration.Params,
		OrganizationId:        t.OrganizationId,
		StartedAt:             startedAt.String(),
	}

	workflows, err := mappers.ExtractWorkflow(t.Integration.Workflow)
	if err != nil {
		return err
	}

	err = t.Store.Insert(ctx, store_integration)
	if err != nil {
		return err
	}

	// store initial status for workflow instance
	err = t.WorkflowInstanceStatusStore.Put(ctx, integrationId, models.WorkflowInstanceStatusEvent{
		Uuid:      integrationId,
		Status:    models.WorkflowInstanceStatusNotStarted,
		Timestamp: int(startedAt.Unix()),
	})
	if err != nil {
		return err
	}

	// store initial status for workflow instance processors
	for _, p := range workflows {
		err = t.WorkflowInstanceStatusStore.Put(ctx, integrationId, models.WorkflowInstanceStatusEvent{
			Uuid:      p.Uuid,
			Status:    models.WorkflowInstanceStatusNotStarted,
			Timestamp: int(startedAt.Unix()),
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

	resp, err := t.Client.Execute(ctx, *bytes.NewBuffer(b))
	// handle responses:
	// currently we expect a 2xx response and no errors?
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println(string(resp))

	return nil
}
