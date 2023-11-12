package trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	Integration models.Integration
	Store       store_dynamodb.DynamoDBStore
}

func NewApplicationTrigger(client clients.Client, application store.Application, integration models.Integration, store store_dynamodb.DynamoDBStore) Trigger {
	return &ApplicationTrigger{client, application, integration, store}
}

// runs trigger
func (t *ApplicationTrigger) Run(ctx context.Context) error {
	fmt.Println("generating uuid")
	id := uuid.New()
	integrationId := id.String()
	fmt.Println(integrationId)

	fmt.Println("persisting to db")
	// persist to dynamodb
	store_integration := store_dynamodb.Integration{
		Uuid:          integrationId,
		ApplicationId: t.Integration.ApplicationID,
		DatasetNodeId: t.Integration.DatasetNodeID,
		PackageIds:    t.Integration.PackageIDs,
	}
	err := t.Store.Insert(ctx, store_integration)
	if err != nil {
		return err
	}

	fmt.Println("creating and marshaling payload")
	applicationPayload := ApplicationPayload{
		IntegrationId: integrationId,
	}
	b, err := json.Marshal(applicationPayload)
	if err != nil {
		return err
	}

	fmt.Println("executing trigger")
	resp, err := t.Client.Execute(context.Background(), *bytes.NewBuffer(b))
	// handle responses:
	// currently we expect a 2xx response and no errors?
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("response ...")
	fmt.Println(string(resp))

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
