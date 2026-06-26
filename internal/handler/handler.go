package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/event_parser"
	"github.com/Pennsieve/integration-service/internal/webhook_mapper"
	"github.com/Pennsieve/integration-service/internal/webhook_sender"
)

func Handler(ctx context.Context, events map[string]interface{}) (map[string]interface{}, error) {
	aws.AwsOnce.Do(func() {
		aws.InitAWS(ctx)
	})

	if err := db.EnsureDB(ctx); err != nil {
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}

	log.Println("Lambda handler invoked at", time.Now())

	mappedEvents, forceRefresh, err := event_parser.MapEvents(events)
	if err != nil {
		return nil, fmt.Errorf("failed to map events: %w", err)
	}

	webhookMessages := webhook_mapper.MapWebhookMessages(ctx, mappedEvents, forceRefresh)
	webhook_sender.BroadcastMessages(webhookMessages)

	return events, nil
}
