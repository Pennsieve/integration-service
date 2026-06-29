package webhook_sender

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/Pennsieve/integration-service/internal/utils"
)

var (
	httpClient = &http.Client{Timeout: 300 * time.Millisecond}
)

const (
	maxRetries   = 3
	retryBackoff = 2 * time.Second
)

func sendWebhookWithRetry(ctx context.Context, url string, body []byte) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Failed to create request for %s (attempt %d): %v", url, attempt, err)
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Error closing response body: %v", err)
			}
			return nil
		}

		if err == nil && resp != nil && resp.StatusCode >= 300 {
			log.Printf("Failed to send request to %s (attempt %d): %v", url, attempt, lastErr)
			lastErr = fmt.Errorf("non-2xx status %d", resp.StatusCode)
		}

		if resp != nil {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Error closing response body: %v", err)
			}
		}
		//TODO: Parallelism to be added as a followup.

		lastErr = err
		if attempt < maxRetries {
			jitter := time.Duration(rand.Int63n(1000)) * time.Millisecond
			backoffDuration := (retryBackoff * time.Duration(attempt)) + jitter
			log.Printf("Attempt %d failed for %s (status: %v): %v. Retrying in %v", attempt, url, resp, err, backoffDuration)
			time.Sleep(backoffDuration)
		}
	}

	return fmt.Errorf("failed to deliver message to %s after %d attempts: %w", url, maxRetries, lastErr)
}

func BroadcastMessages(ctx context.Context, messages map[string]models.WebhookMessage) {
	for _, record := range messages {
		urlSet := make(map[string]bool)
		for _, url := range record.URLs {
			urlSet[url] = true
		}

		for url := range urlSet {
			for _, msg := range record.Messages {
				body, err := utils.WebhookBodyParser(url, msg)
				if err != nil {
					log.Printf("Failed to parse webhook body for %s: %v", url, err)
					continue
				}

				if err := sendWebhookWithRetry(ctx, url, body); err != nil {
					log.Printf("Failed to send webhook: %v", err)
					continue
				}
			}
		}
	}
}
