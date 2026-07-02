package dbmigrate

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/pennsieve/dbmigrate-go/pkg/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func ConfigDefaults() config.DefaultSettings {
	return config.DefaultSettings{config.PostgresSchemaKey: "webhooks"}
}

func MigrationsSource() (source.Driver, error) {
	migrationSource, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("error creating migration source.Driver: %w", err)
	}
	return migrationSource, nil
}
