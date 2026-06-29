package webhook_mapper

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Pennsieve/integration-service/internal/cache"
	"github.com/Pennsieve/integration-service/internal/models"
)

const (
	cacheExpiration = 10 * time.Minute
)

func MapWebhookMessages(ctx context.Context, mapped map[string][]models.EventMessage, forceRefresh bool) map[string]models.WebhookMessage {
	result := make(map[string]models.WebhookMessage)

	for orgID, events := range mapped {
		cacheEntry, exists := cache.Get(orgID)

		if forceRefresh || !exists || time.Since(cacheEntry.Updated) > cacheExpiration {
			cache.RefreshWebhookCache(ctx, orgID)
			cacheEntry, exists = cache.Get(orgID)
		}

		if !exists || len(cacheEntry.Webhooks) == 0 {
			log.Printf("No webhooks found for org %s\n", orgID)
			continue
		}

		webhookLookup := buildWebhookLookup(cacheEntry.Webhooks)

		for _, evt := range events {
			key := fmt.Sprintf("%d:%s", evt.DataID, evt.Category)

			entry := result[key]
			entry.Messages = append(entry.Messages, evt)

			if urls, ok := webhookLookup[key]; ok {
				entry.URLs = append(entry.URLs, urls...)
			}

			result[key] = entry
		}
	}

	return result
}
func buildWebhookLookup(webhooks []models.WebhookRecord) map[string][]string {
	lookup := make(map[string][]string)
	for _, w := range webhooks {
		key := fmt.Sprintf("%d:%s", w.DatasetID, w.EventName)
		lookup[key] = append(lookup[key], w.APIURL)
	}
	return lookup
}
