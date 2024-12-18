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

func TestExtractParam(t *testing.T) {
	request := events.APIGatewayV2HTTPRequest{
		RouteKey: "GET /integrations/someintegrationId",
		Body:     "{ \"datasetId\": \"dataset123\", \"applicationId\": 1, \"packageIds\": [\"1\"]}",
		RawPath:  "/integrations/someintegrationId",
	}
	expected := "someintegrationId"
	got := utils.ExtractParam(request.RawPath)
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestConvertEpochToUTCLocation(t *testing.T) {
	epoch := int64(1734545552324)
	expected := "2024-12-18 18:12:32 +0000 UTC"
	got := utils.ConvertEpochToUTCLocation(int64(epoch))
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestConvertEpochToUTCRFC3339(t *testing.T) {
	epoch := int64(1734545552324)
	expected := "2024-12-18T18:12:32Z"
	got := utils.ConvertEpochToUTCRFC3339(int64(epoch))
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
