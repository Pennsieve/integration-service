package mappers_test

import (
	"testing"

	"github.com/pennsieve/integration-service/service/mappers"
	"github.com/stretchr/testify/assert"
)

func TestServiceResponseToAuxiliaryResponse(t *testing.T) {
	resp := []byte(`{"messages": [{"timestamp": 1734545552324, "message": "start of processing","ingestionTime": 1734545557240}]}`)

	result, _ := mappers.ServiceResponseToAuxiliaryResponse(resp)
	assert.Equal(t, "2024-12-18 18:12:32 +0000 UTC", result.Messages[0].UtcLocationTime)
	assert.Equal(t, "2024-12-18T18:12:32Z", result.Messages[0].UtcTime)
}
