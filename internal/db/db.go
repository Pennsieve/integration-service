package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/models"

	// Registers the "postgres" driver with database/sql via its init().
	// database/sql is only an abstraction layer; without a concrete driver
	// imported for its side-effect, sql.Open("postgres", ...) fails at runtime
	// with: sql: unknown driver "postgres" (forgotten import?).
	_ "github.com/lib/pq"
)

var (
	env       = os.Getenv("ENV")
	dbPool    *sql.DB
	dbOnce    sync.Once
	dbInitErr error
)

const (
	dbMaxOpenConns    = 5
	dbMaxIdleConns    = 2
	dbConnMaxLifetime = 5 * time.Minute
)

func initDB(ctx context.Context) error {
	dbname, dberr := aws.GetSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-db", env), false)
	if dberr != nil {
		return fmt.Errorf("failed to get DB name: %w", dberr)
	}
	dbusername, usererr := aws.GetSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-user", env), false)
	if usererr != nil {
		return fmt.Errorf("failed to get DB username: %w", usererr)
	}
	dbpassword, pwerr := aws.GetSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-password", env), true)
	if pwerr != nil {
		return fmt.Errorf("failed to get DB password: %w", pwerr)
	}
	dbhostname, hosterr := aws.GetSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-host", env), false)
	if hosterr != nil {
		return fmt.Errorf("failed to get DB hostname: %w", hosterr)
	}

	connectionStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=require",
		dbhostname, dbusername, dbpassword, dbname)

	db, err := sql.Open("postgres", connectionStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)
	db.SetConnMaxLifetime(dbConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	dbPool = db
	return nil
}

func EnsureDB(ctx context.Context) error {
	dbOnce.Do(func() {
		dbInitErr = initDB(ctx)
	})
	return dbInitErr
}

func Query(ctx context.Context, command string) ([]models.WebhookRecord, error) {
	rows, err := dbPool.QueryContext(ctx, command)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.WebhookRecord
	for rows.Next() {
		var r models.WebhookRecord
		if err := rows.Scan(&r.APIURL, &r.EventName, &r.DatasetID); err != nil {
			return nil, err
		}
		res = append(res, r)
	}

	return res, rows.Err()
}
