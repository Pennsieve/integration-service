package log_retriever_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/pennsieve/integration-service/service/clients"
	"github.com/pennsieve/integration-service/service/log_retriever"
)

func TestRun(t *testing.T) {
	// mockClient := mocks.NewMockClient()
	httpClient := clients.NewComputeRestClient(&http.Client{}, "https://mz23hnbrxrhutrwm2ay7j5ue4u0lbfqe.lambda-url.us-east-1.on.aws/logs")
	retriever := log_retriever.NewLogRetriever(httpClient,
		"d25c7b47-de21-4009-a311-bdf950d05a50",
		"801a4665-ba47-4053-9071-8503efb063ca")
	ctx := context.Background()
	resp, err := retriever.Run(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(resp))
}
