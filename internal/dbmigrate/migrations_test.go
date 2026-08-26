package dbmigrate_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	integrationdbmigrate "github.com/Pennsieve/integration-service/internal/dbmigrate"
	"github.com/golang-migrate/migrate/v4/source"
	_ "github.com/lib/pq"
	dbmigrateconfig "github.com/pennsieve/dbmigrate-go/pkg/config"
	"github.com/pennsieve/dbmigrate-go/pkg/dbmigrate"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	seedImage            = "pennsieve/pennsievedb:V20241120161735-seed"
	postgresUser         = "postgres"
	postgresPassword     = "password"
	postgresDatabase     = "postgres"
	containerStartupWait = 2 * time.Minute
)

// TestMigrations starts a throwaway Postgres container from the same seed
// image used in production (pennsieve.users already present), runs this
// repo's webhooks and notifications migrations against it via the same
// MigrationsSource/Config builders cmd/dbmigrate uses, and asserts against
// the real resulting schema.
func TestMigrations(t *testing.T) {
	ctx := context.Background()

	container, err := testcontainers.Run(ctx, seedImage,
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     postgresUser,
			"POSTGRES_PASSWORD": postgresPassword,
			"POSTGRES_DB":       postgresDatabase,
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(containerStartupWait),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, container.Terminate(ctx))
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	runMigrationsUp(t, ctx, host, port.Port(), "webhooks", integrationdbmigrate.WebhooksConfigDefaults(), integrationdbmigrate.WebhooksMigrationsSource)
	runMigrationsUp(t, ctx, host, port.Port(), "notifications", integrationdbmigrate.NotificationsConfigDefaults(), integrationdbmigrate.NotificationsMigrationsSource)

	db, err := sql.Open("postgres", datasourceName(host, port.Port(), "public"))
	require.NoError(t, err)
	defer db.Close()

	t.Run("webhooks.messages accepts a row", func(t *testing.T) {
		var id int
		err := db.QueryRow(
			`INSERT INTO webhooks.messages (payload) VALUES ($1) RETURNING id`,
			`{"event":"test"}`,
		).Scan(&id)
		require.NoError(t, err)
		require.NotZero(t, id)
	})

	t.Run("webhooks.sender_rate_limits accepts a row", func(t *testing.T) {
		_, err := db.Exec(
			`INSERT INTO webhooks.sender_rate_limits (sender_ip, window_start, request_count) VALUES ($1, now(), 1)`,
			"127.0.0.1",
		)
		require.NoError(t, err)
	})

	var seededUserID int
	err = db.QueryRow(`SELECT id FROM pennsieve.users ORDER BY id LIMIT 1`).Scan(&seededUserID)
	require.NoError(t, err, "expected the seed image to already contain at least one pennsieve.users row")

	t.Run("notifications.subscriptions enforces the topics FK and unique constraint", func(t *testing.T) {
		var topicID int
		err := db.QueryRow(
			`INSERT INTO notifications.topics (name) VALUES ($1) RETURNING topic_id`,
			"test-topic",
		).Scan(&topicID)
		require.NoError(t, err)

		var subscriptionID int
		err = db.QueryRow(
			`INSERT INTO notifications.subscriptions (user_id, topic_id) VALUES ($1, $2) RETURNING subscription_id`,
			seededUserID, topicID,
		).Scan(&subscriptionID)
		require.NoError(t, err)
		require.NotZero(t, subscriptionID)

		// duplicate (user_id, topic_id, context) must be rejected
		_, err = db.Exec(
			`INSERT INTO notifications.subscriptions (user_id, topic_id) VALUES ($1, $2)`,
			seededUserID, topicID,
		)
		require.Error(t, err)

		// FK violation on a non-existent topic must be rejected
		_, err = db.Exec(
			`INSERT INTO notifications.subscriptions (user_id, topic_id) VALUES ($1, $2)`,
			seededUserID, topicID+1_000_000,
		)
		require.Error(t, err)
	})

	t.Run("notifications.user_notifications enforces the status CHECK constraint", func(t *testing.T) {
		var topicID int
		err := db.QueryRow(
			`INSERT INTO notifications.topics (name) VALUES ($1) RETURNING topic_id`,
			"test-topic-status",
		).Scan(&topicID)
		require.NoError(t, err)

		var subscriptionID int
		err = db.QueryRow(
			`INSERT INTO notifications.subscriptions (user_id, topic_id) VALUES ($1, $2) RETURNING subscription_id`,
			seededUserID, topicID,
		).Scan(&subscriptionID)
		require.NoError(t, err)

		var notificationID int
		err = db.QueryRow(
			`INSERT INTO notifications.notifications (subscription_id, title, message) VALUES ($1, $2, $3) RETURNING notification_id`,
			subscriptionID, "title", "message",
		).Scan(&notificationID)
		require.NoError(t, err)

		_, err = db.Exec(
			`INSERT INTO notifications.user_notifications (user_id, notification_id, status) VALUES ($1, $2, 'UNREAD')`,
			seededUserID, notificationID,
		)
		require.NoError(t, err)

		_, err = db.Exec(
			`INSERT INTO notifications.user_notifications (user_id, notification_id, status) VALUES ($1, $2, 'BOGUS')`,
			seededUserID, notificationID+1_000_000,
		)
		require.Error(t, err)
	})
}

func runMigrationsUp(t *testing.T, ctx context.Context, host, port, schema string, defaults dbmigrateconfig.DefaultSettings, migrationsSource func() (source.Driver, error)) {
	t.Helper()

	password := postgresPassword
	migrateConfig := dbmigrateconfig.Config{
		PostgresDB: dbmigrateconfig.PostgresDBConfig{
			Host:     host,
			Port:     mustAtoi(t, port),
			User:     postgresUser,
			Password: &password,
			Database: postgresDatabase,
			Schema:   defaults[dbmigrateconfig.PostgresSchemaKey],
		},
		VerboseLogging: true,
	}

	src, err := migrationsSource()
	require.NoError(t, err)

	m, err := dbmigrate.NewLocalMigrator(ctx, migrateConfig, src)
	require.NoError(t, err)
	defer m.CloseAndLogError()

	require.NoError(t, m.Up(), "running %s migrations up", schema)
}

func datasourceName(host, port, searchPath string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		postgresUser, postgresPassword, host, port, postgresDatabase, searchPath)
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	require.NoError(t, err)
	return n
}
