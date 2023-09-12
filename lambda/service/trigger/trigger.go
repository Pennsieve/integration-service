package trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/store"
)

type Trigger interface {
	Run(ctx context.Context) error
	Validate() error
}

type ApplicationTrigger struct {
	Client      clients.Client
	Application store.Application
	Params      interface{}
}

func NewApplicationTrigger(client clients.Client, application store.Application, params interface{}) Trigger {
	return &ApplicationTrigger{client, application, params}
}

// runs trigger
func (t *ApplicationTrigger) Run(ctx context.Context) error {
	// TODO: update to pass integrationId once DB is setup
	b, err := json.Marshal(t.Params)
	if err != nil {
		return err
	}
	_, err = t.Client.Execute(ctx, *bytes.NewBuffer(b))
	// handle responses:
	// currently we expect a 2xx response and no errors?
	if err != nil {
		return err
	}
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
