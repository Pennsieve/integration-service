package handler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/email"
	"github.com/Pennsieve/integration-service/internal/event_parser"
	"github.com/Pennsieve/integration-service/internal/notification_generator"
	"github.com/Pennsieve/integration-service/internal/webhook_mapper"
	"github.com/Pennsieve/integration-service/internal/webhook_sender"
)

var (
	emailSenderOnce sync.Once
	emailSender     email.Sender
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

	// Built once per container, after InitAWS. A sender that can't be built
	// leaves emailSender nil, so notifications are still recorded and only
	// their emails are skipped (and logged as such by Generate).
	emailSenderOnce.Do(func() {
		sender, err := email.NewSender()
		if err != nil {
			log.Printf("ERROR email sender init: %v", err)
			return
		}
		emailSender = sender
	})
	notification_generator.Generate(ctx, mappedEvents, emailSender)

	webhookMessages := webhook_mapper.MapWebhookMessages(ctx, mappedEvents, forceRefresh)
	webhook_sender.BroadcastMessages(ctx, webhookMessages)

	return events, nil
}
