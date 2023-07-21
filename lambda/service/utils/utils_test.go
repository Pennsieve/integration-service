package utils_test

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/utils"
)

func TestExtractRouteKey(t *testing.T) {
	request := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body:     "{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetI\": \"dataset123\", \"applicationId\": 1, \"payload\": {\"packageIds\": [1,2,3]}}",
	}
	expected := "/integrations"
	got := utils.ExtractRoute(request.RouteKey)
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
