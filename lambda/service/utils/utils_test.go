package utils_test

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/utils"
)

func TestExtractRouteKey(t *testing.T) {
	request := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body:     "{ \"datasetId\": \"dataset123\", \"applicationId\": 1, \"packageIds\": [\"1\"]}",
	}
	expected := "/integrations"
	got := utils.ExtractRoute(request.RouteKey)
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
