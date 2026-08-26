package dbmigrate_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestMigrations asserts against the real schema produced by this repo's own
// dbmigrate image. docker-compose.test.yml runs that image against
// POSTGRES_HOST (see the dbmigrate service) before the test binary starts,
// so by the time this test runs both the webhooks and notifications schemas
// are already migrated.
func TestMigrations(t *testing.T) {
	db, err := sql.Open("postgres", datasourceName())
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

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

func datasourceName() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DATABASE"),
	)
}
