package trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
)

type Trigger interface {
	Run(ctx context.Context) error
	Validate() error
}

type ApplicationTrigger struct {
	Client      clients.Client
	Application models.Application
	Payload     models.TriggerPayload
}

func NewApplicationTrigger(client clients.Client, application models.Application, payload models.TriggerPayload) Trigger {
	return &ApplicationTrigger{client, application, payload}
}

// runs trigger
func (t *ApplicationTrigger) Run(ctx context.Context) error {
	b, err := json.Marshal(t.Payload)
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
	if !t.Application.IsActive {
		err := errors.New("application should be active")
		log.Println(err)
		return err
	}
	return nil
}
