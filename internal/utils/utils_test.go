package utils

import (
	"encoding/json"
	"testing"

	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookBodyParser_DefaultIsRawJSON(t *testing.T) {
	msg := models.EventMessage{OrgID: "org1", DataID: 7, Category: "FILES", Type: "UPLOAD"}

	body, err := WebhookBodyParser("https://example.com/hook", msg)
	require.NoError(t, err)

	var got models.EventMessage
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, msg, got)
}

func TestWebhookBodyParser_SlackWrapsInTextField(t *testing.T) {
	msg := models.EventMessage{OrgID: "org1", DataID: 7, Category: "FILES", Type: "UPLOAD"}

	body, err := WebhookBodyParser("https://hooks.slack.com/services/T000/B000/xxx", msg)
	require.NoError(t, err)

	// Slack payloads are {"text": "<json string>"} — the value is the event
	// re-encoded as a string, matching the Python slack_parser.
	var wrapper map[string]string
	require.NoError(t, json.Unmarshal(body, &wrapper))
	assert.Contains(t, wrapper, "text")

	var inner models.EventMessage
	require.NoError(t, json.Unmarshal([]byte(wrapper["text"]), &inner))
	assert.Equal(t, msg, inner)
}
