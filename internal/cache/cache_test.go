package cache

import (
	"context"
	"testing"
	"time"

	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestGetSet_RoundTrips(t *testing.T) {
	entry := models.WebhookCache{
		Updated:  time.Now(),
		Webhooks: []models.WebhookRecord{{APIURL: "https://x.example", EventName: "FILES", DatasetID: 1}},
	}
	Set("orgRoundTrip", entry)

	got, ok := Get("orgRoundTrip")
	assert.True(t, ok)
	assert.Equal(t, entry.Webhooks, got.Webhooks)

	_, ok = Get("never-set")
	assert.False(t, ok)
}

// The schema name is interpolated into the SQL string (it is an identifier, not
// a bindable value), so RefreshWebhookCache guards orgID against a strict
// allowlist before building any query. An injection attempt must be rejected
// before a DB call is ever made — which is why this passes without a DB.
func TestRefreshWebhookCache_RejectsInvalidOrgID(t *testing.T) {
	for _, bad := range []string{
		`org"; DROP TABLE webhooks; --`,
		"org with spaces",
		"org-with-dashes",
		"",
	} {
		// Must not panic or attempt a query; just logs and returns.
		RefreshWebhookCache(context.Background(), bad)
		_, ok := Get(bad)
		assert.False(t, ok, "invalid orgID %q must not produce a cache entry", bad)
	}
}

func TestOrgIDPattern_AcceptsValid(t *testing.T) {
	for _, good := range []string{"org1", "N_organization_123", "ABC"} {
		assert.True(t, orgIDPattern.MatchString(good), "expected %q to be valid", good)
	}
}
