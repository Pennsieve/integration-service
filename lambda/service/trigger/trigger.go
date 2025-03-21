package trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/integration-service/service/store_dynamodb"
)

type Trigger interface {
	Run(ctx context.Context) error
	Validate() error
}

type ApplicationTrigger struct {
	Client      clients.Client
	Application store.Application
	Integration models.WorkflowInstance
	Store       store_dynamodb.DynamoDBStore
}

func NewApplicationTrigger(client clients.Client, application store.Application, integration models.WorkflowInstance, store store_dynamodb.DynamoDBStore) Trigger {
	return &ApplicationTrigger{client, application, integration, store}
}

// runs trigger
func (t *ApplicationTrigger) Run(ctx context.Context) error {
	id := uuid.New()
	integrationId := id.String()

	// persist to dynamodb
	store_integration := store_dynamodb.WorkflowInstance{
		Uuid:          integrationId,
		DatasetNodeId: t.Integration.DatasetNodeID,
		PackageIds:    t.Integration.PackageIDs,
		Params:        t.Integration.Params,
	}
	err := t.Store.Insert(ctx, store_integration)
	if err != nil {
		return err
	}

	applicationPayload := ApplicationPayload{
		IntegrationId: integrationId,
	}
	b, err := json.Marshal(applicationPayload)
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

// validates whether a trigger can be executed
func (t *ApplicationTrigger) Validate() error {
	if t.Application.IsDisabled {
		err := errors.New("application should be active")
		return err
	}
	return nil
}
