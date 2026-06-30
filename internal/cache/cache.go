package cache

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
)

// This package is the single owner of the webhook cache. Both the writer
// (RefreshWebhookCache) and the reader (Get) operate on this one map, guarded
// by this one mutex. Previously webhook_mapper declared its own separate
// webhookCache var, so refreshes never reached the reader and every org looked
// empty.
var (
	webhookCache = make(map[string]models.WebhookCache)
	cacheMutex   sync.RWMutex
	orgIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

// webhookQuery is the verbatim port of the Python refresh_webhook_cache query.
// The schema name is interpolated (it is a Postgres identifier, not a value, so
// it cannot be parameterized), which is why orgID is validated against
// orgIDPattern before we ever build this string. Column order here must match
// the rows.Scan order in db.Query: api_url, event_name, dataset_id.
const webhookQuery = `SELECT wh.api_url, wet.event_name, wi.dataset_id
FROM "%[1]s".webhooks AS wh
INNER JOIN "%[1]s".webhook_event_subscriptions AS wes ON wh.id = wes.webhook_id
INNER JOIN "%[1]s".dataset_integrations AS wi ON wh.id = wi.webhook_id
INNER JOIN "%[1]s".webhook_event_types AS wet ON wes.webhook_event_type_id = wet.id`

// Get returns the cached webhook entry for an org and whether it exists.
// Reads through the same mutex the writer uses.
func Get(orgID string) (models.WebhookCache, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	entry, ok := webhookCache[orgID]
	return entry, ok
}

// Set replaces the cached entry for an org. Primarily a seam for tests that
// want to pre-seed the cache so downstream logic can be exercised without a DB.
func Set(orgID string, entry models.WebhookCache) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	webhookCache[orgID] = entry
}

func RefreshWebhookCache(ctx context.Context, orgID string) {
	if !orgIDPattern.MatchString(orgID) {
		log.Printf("invalid org id: %q\n", orgID)
		return
	}

	command := fmt.Sprintf(webhookQuery, orgID)

	results, err := db.Query(ctx, command)
	if err != nil {
		log.Printf("SQL error: %v\n", err)
		return
	}

	cacheMutex.Lock()
	webhookCache[orgID] = models.WebhookCache{
		Updated:  time.Now(),
		Webhooks: results,
	}
	cacheMutex.Unlock()
}
