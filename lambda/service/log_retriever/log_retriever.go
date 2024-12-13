package log_retriever

import (
	"context"
	"log"

	"github.com/pennsieve/integration-service/service/clients"
)

type Retriever interface {
	Run(context.Context) ([]byte, error)
}

type LogRetriever struct {
	Client             clients.Client
	WorkflowInstanceId string
	ApplicationUuid    string
}

func NewLogRetriever(client clients.Client, uuid string, applicationUuid string) Retriever {
	return &LogRetriever{client, uuid, applicationUuid}
}

// runs retriever
func (t *LogRetriever) Run(ctx context.Context) ([]byte, error) {
	params := map[string]string{
		"integrationId":   t.WorkflowInstanceId,
		"applicationUuid": t.ApplicationUuid,
	}
	resp, err := t.Client.Retrieve(ctx, params)
	// handle responses:
	// currently we expect a 2xx response and no errors?
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return resp, nil
}
