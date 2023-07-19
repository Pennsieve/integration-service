package trigger_test

import (
	"net/http"
	"testing"

	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/trigger"
)

func TestRun(t *testing.T) {
	application := models.Application{
		ID:         1,
		Name:       "mockApplication",
		URL:        "http://localhost:8081/mock",
		IsActive:   true,
		IsInternal: false,
	}
	triggerPayload := models.TriggerPayload{
		PackageIDs: []int64{1, 2, 3},
	}
	client := clients.NewApplicationRestClient(&http.Client{}, application.URL)
	applicationTrigger := trigger.NewApplicationTrigger(client,
		application,
		triggerPayload)
	err := applicationTrigger.Run()
	if err != nil {
		t.Error(err)
	}
}
