package log_retriever

import (
	"context"
	"log"
	"os"

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

	var resp []byte
	var respError error
	authenticationMode := os.Getenv("COMPUTE_GATEWAY_AUTHENTICATION_TYPE")
	if authenticationMode == "IAM" {
		resp, respError = t.Client.Retrieve(ctx, params)
	} else {
		resp, respError = t.Client.RetrieveLegacy(ctx, params)
	}

	// handle responses:
	// currently we expect a 2xx response and no errors?
	if respError != nil {
		log.Println(respError.Error())
		return nil, respError
	}

	return resp, nil
}
