package trigger

import "github.com/pennsieve/integration-service/service/models"

type Trigger interface {
	Run() error
}

type ApplicationTrigger struct {
	Application models.Application
	Payload     models.TriggerPayload
}

func NewApplicationTrigger(application models.Application, payload models.TriggerPayload) Trigger {
	return &ApplicationTrigger{application, payload}
}

func (t *ApplicationTrigger) Run() error {
	return nil
}
