package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	integrationdbmigrate "github.com/Pennsieve/integration-service/internal/dbmigrate"
	"github.com/golang-migrate/migrate/v4/source"
	dbmigrateconfig "github.com/pennsieve/dbmigrate-go/pkg/config"
	"github.com/pennsieve/dbmigrate-go/pkg/dbmigrate"
)

var logger = slog.Default()

func main() {
	ctx := context.Background()

	if err := runMigrations(ctx, "webhooks", integrationdbmigrate.WebhooksConfigDefaults(), integrationdbmigrate.WebhooksMigrationsSource); err != nil {
		logger.Error("error running webhooks migrations", slog.Any("error", err))
		os.Exit(1)
	}

	if err := runMigrations(ctx, "notifications", integrationdbmigrate.NotificationsConfigDefaults(), integrationdbmigrate.NotificationsMigrationsSource); err != nil {
		logger.Error("error running notifications migrations", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("integration-service DB schema migration complete")
}

// runMigrations loads config for a single schema and runs its migrations up.
// Called once per domain schema (webhooks, notifications) so a bad
// config/migration for one schema can't be masked by the other.
func runMigrations(ctx context.Context, name string, defaults dbmigrateconfig.DefaultSettings, migrationsSource func() (source.Driver, error)) error {
	migrateConfig, err := dbmigrateconfig.LoadConfig(defaults)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	if migrateConfig.PostgresDB.Password == nil {
		return fmt.Errorf("password must be specified; cannot currently use RDS proxy for migrates since no Postgres role with the appropriate grants has credentials in the proxy")
	}

	logger.
		With(slog.String("schema", name),
			slog.Bool("verboseLogging", migrateConfig.VerboseLogging),
			slog.Group("postgres",
				slog.String("host", migrateConfig.PostgresDB.Host),
				slog.Int("port", migrateConfig.PostgresDB.Port),
				slog.String("username", migrateConfig.PostgresDB.User),
				slog.String("database", migrateConfig.PostgresDB.Database),
				slog.String("schema", migrateConfig.PostgresDB.Schema),
			)).
		Info("integration-service DB schema migration started")

	source, err := migrationsSource()
	if err != nil {
		return fmt.Errorf("error creating MigrationsSource: %w", err)
	}

	m, err := dbmigrate.NewLocalMigrator(ctx, migrateConfig, source)
	if err != nil {
		return fmt.Errorf("error creating DatabaseMigrator: %w", err)
	}
	defer m.CloseAndLogError()

	if err := m.Up(); err != nil {
		return fmt.Errorf("error running 'up' migrations: %w", err)
	}

	logger.Info("integration-service DB schema migration complete", slog.String("schema", name))
	return nil
}
