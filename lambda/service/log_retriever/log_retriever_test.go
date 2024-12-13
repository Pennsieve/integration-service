package log_retriever_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pennsieve/integration-service/service/log_retriever"
	"github.com/pennsieve/integration-service/service/mocks"
)

func TestRun(t *testing.T) {
	mockClient := mocks.NewMockClient()
	retriever := log_retriever.NewLogRetriever(mockClient,
		"someWorkflowInstanceId",
		"someApplicationUUid")
	ctx := context.Background()
	_, err := retriever.Run(ctx)
	if err != nil {
		fmt.Println(err)
	}
}
