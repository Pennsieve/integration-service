package trigger

import (
	"bytes"
	"encoding/json"

	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
)

type Trigger interface {
	Run() error
}

type ApplicationTrigger struct {
	Client      clients.Client
	Application models.Application
	Payload     models.TriggerPayload
}

func NewApplicationTrigger(client clients.Client, application models.Application, payload models.TriggerPayload) Trigger {
	return &ApplicationTrigger{client, application, payload}
}

func (t *ApplicationTrigger) Run() error {
	b, err := json.Marshal(t.Payload)
	if err != nil {
		return err
	}
	_, err = t.Client.Execute(*bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	return nil
}
