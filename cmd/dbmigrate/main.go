package main

import (
	"context"
	"log/slog"
	"os"

	integrationdbmigrate "github.com/Pennsieve/integration-service/internal/dbmigrate"
	dbmigrateconfig "github.com/pennsieve/dbmigrate-go/pkg/config"
	"github.com/pennsieve/dbmigrate-go/pkg/dbmigrate"
)

var logger = slog.Default()

func main() {
	ctx := context.Background()

	migrateConfig, err := dbmigrateconfig.LoadConfig(integrationdbmigrate.ConfigDefaults())
	if err != nil {
		logger.Error("error loading config", slog.Any("error", err))
		os.Exit(1)
	}

	if migrateConfig.PostgresDB.Password == nil {
		logger.Error("password must be specified; cannot currently use RDS proxy for migrates since no Postgres role with the appropriate grants has credentials in the proxy")
		os.Exit(1)
	}

	logger.
		With(slog.Bool("verboseLogging", migrateConfig.VerboseLogging),
			slog.Group("postgres",
				slog.String("host", migrateConfig.PostgresDB.Host),
				slog.Int("port", migrateConfig.PostgresDB.Port),
				slog.String("username", migrateConfig.PostgresDB.User),
				slog.String("database", migrateConfig.PostgresDB.Database),
				slog.String("schema", migrateConfig.PostgresDB.Schema),
			)).
		Info("integration-service DB schema migration started")

	migrationsSource, err := integrationdbmigrate.MigrationsSource()
	if err != nil {
		logger.Error("error creating integration-service MigrationsSource", slog.Any("error", err))
		os.Exit(1)
	}

	m, err := dbmigrate.NewLocalMigrator(ctx, migrateConfig, migrationsSource)
	if err != nil {
		logger.Error("error creating DatabaseMigrator", slog.Any("error", err))
		os.Exit(1)
	}
	defer m.CloseAndLogError()

	if err := m.Up(); err != nil {
		logger.Error("error running 'up' migrations", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("integration-service DB schema migration complete")
}
