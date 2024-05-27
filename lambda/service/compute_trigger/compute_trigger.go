package compute_trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

type Trigger interface {
	Run(ctx context.Context) error
}

type ComputeTrigger struct {
	Client      clients.Client
	Integration models.Integration
	Store       store_dynamodb.DynamoDBStore
}

func NewComputeTrigger(client clients.Client, integration models.Integration, store store_dynamodb.DynamoDBStore) Trigger {
	return &ComputeTrigger{client, integration, store}
}

// runs trigger
func (t *ComputeTrigger) Run(ctx context.Context) error {
	id := uuid.New()
	integrationId := id.String()

	// persist to dynamodb
	store_integration := store_dynamodb.Integration{
		Uuid:            integrationId,
		ComputeNodeUuid: t.Integration.ComputeNode.ComputeNodeUuid,
		DatasetNodeId:   t.Integration.DatasetNodeID,
		PackageIds:      t.Integration.PackageIDs,
		Workflow:        t.Integration.Workflow,
		Params:          t.Integration.Params,
	}
	err := t.Store.Insert(ctx, store_integration)
	if err != nil {
		return err
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
