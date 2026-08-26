package dbmigrate

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/pennsieve/dbmigrate-go/pkg/config"
)

//go:embed migrations/webhooks/*.sql
var webhooksMigrationsFS embed.FS

//go:embed migrations/notifications/*.sql
var notificationsMigrationsFS embed.FS

// WebhooksConfigDefaults returns the config defaults for running migrations
// against the webhooks schema.
func WebhooksConfigDefaults() config.DefaultSettings {
	return config.DefaultSettings{config.PostgresSchemaKey: "webhooks"}
}

// NotificationsConfigDefaults returns the config defaults for running
// migrations against the notifications schema.
func NotificationsConfigDefaults() config.DefaultSettings {
	return config.DefaultSettings{config.PostgresSchemaKey: "notifications"}
}

// WebhooksMigrationsSource builds the migration source.Driver for the
// webhooks schema's embedded SQL files.
func WebhooksMigrationsSource() (source.Driver, error) {
	migrationSource, err := iofs.New(webhooksMigrationsFS, "migrations/webhooks")
	if err != nil {
		return nil, fmt.Errorf("error creating webhooks MigrationsSource: %w", err)
	}
	return migrationSource, nil
}

// NotificationsMigrationsSource builds the migration source.Driver for the
// notifications schema's embedded SQL files.
func NotificationsMigrationsSource() (source.Driver, error) {
	migrationSource, err := iofs.New(notificationsMigrationsFS, "migrations/notifications")
	if err != nil {
		return nil, fmt.Errorf("error creating notifications MigrationsSource: %w", err)
	}
	return migrationSource, nil
}
