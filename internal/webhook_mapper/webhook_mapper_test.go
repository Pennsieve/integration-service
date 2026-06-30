package webhook_mapper

import (
	"context"
	"testing"
	"time"

	"github.com/Pennsieve/integration-service/internal/cache"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWebhookLookup_KeysByDatasetAndEvent(t *testing.T) {
	lookup := buildWebhookLookup([]models.WebhookRecord{
		{APIURL: "https://a.example/hook", EventName: "FILES", DatasetID: 1},
		{APIURL: "https://b.example/hook", EventName: "FILES", DatasetID: 1},
		{APIURL: "https://c.example/hook", EventName: "METADATA", DatasetID: 2},
	})

	assert.ElementsMatch(t, []string{"https://a.example/hook", "https://b.example/hook"}, lookup["1:FILES"])
	assert.Equal(t, []string{"https://c.example/hook"}, lookup["2:METADATA"])
}

// Pre-seeding the cache with a fresh timestamp and forceRefresh=false means the
// refresh path (which would hit the DB) is skipped, so MapWebhookMessages can be
// exercised end-to-end with no database. This test would have failed before the
// cache-map unification, since the mapper read a different map than the cache wrote.
func TestMapWebhookMessages_MatchesEventsToURLs(t *testing.T) {
	cache.Set("org1", models.WebhookCache{
		Updated: time.Now(),
		Webhooks: []models.WebhookRecord{
			{APIURL: "https://a.example/hook", EventName: "FILES", DatasetID: 1},
		},
	})

	mapped := map[string][]models.EventMessage{
		"org1": {{OrgID: "org1", DataID: 1, Category: "FILES", Type: "UPLOAD"}},
	}

	result := MapWebhookMessages(context.Background(), mapped, false)

	require.Contains(t, result, "1:FILES")
	assert.Equal(t, []string{"https://a.example/hook"}, result["1:FILES"].URLs)
	assert.Len(t, result["1:FILES"].Messages, 1)
}

func TestMapWebhookMessages_NoMatchingWebhookYieldsNoURLs(t *testing.T) {
	cache.Set("org2", models.WebhookCache{
		Updated: time.Now(),
		Webhooks: []models.WebhookRecord{
			{APIURL: "https://a.example/hook", EventName: "METADATA", DatasetID: 99},
		},
	})

	mapped := map[string][]models.EventMessage{
		"org2": {{OrgID: "org2", DataID: 1, Category: "FILES", Type: "UPLOAD"}},
	}

	result := MapWebhookMessages(context.Background(), mapped, false)

	// The event is still recorded, but with no URLs since nothing subscribed.
	require.Contains(t, result, "1:FILES")
	assert.Empty(t, result["1:FILES"].URLs)
}
