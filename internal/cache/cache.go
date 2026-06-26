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

var (
	webhookCache = make(map[string]models.WebhookCache)
	cacheMutex   sync.RWMutex
	orgIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

func RefreshWebhookCache(ctx context.Context, orgID string) {
	if !orgIDPattern.MatchString(orgID) {
		log.Printf("invalid org id: %q\n", orgID)
		return
	}

	command := fmt.Sprintf(`SELECT ... FROM "%s"...`, orgID)

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
