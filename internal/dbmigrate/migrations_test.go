package dbmigrate_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

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

	t.Run("pennsieve.users carries the columns notification emails are addressed from", func(t *testing.T) {
		// db.GetEmailRecipients reads these; the table belongs to the
		// Pennsieve API, so this catches it drifting from what we query.
		var email string
		var firstName, lastName sql.NullString
		err := db.QueryRow(
			`SELECT email, first_name, last_name FROM pennsieve.users WHERE id = $1`, seededUserID,
		).Scan(&email, &firstName, &lastName)
		require.NoError(t, err)
	})

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

	t.Run("notifications.topics and subscriptions default to enabled and can be disabled", func(t *testing.T) {
		var topicID int
		var topicEnabled bool
		err := db.QueryRow(
			`INSERT INTO notifications.topics (name) VALUES ($1) RETURNING topic_id, enabled`,
			"test-topic-enabled",
		).Scan(&topicID, &topicEnabled)
		require.NoError(t, err)
		require.True(t, topicEnabled)

		var subscriptionID int
		var subscriptionEnabled bool
		err = db.QueryRow(
			`INSERT INTO notifications.subscriptions (user_id, topic_id) VALUES ($1, $2) RETURNING subscription_id, enabled`,
			seededUserID, topicID,
		).Scan(&subscriptionID, &subscriptionEnabled)
		require.NoError(t, err)
		require.True(t, subscriptionEnabled)

		_, err = db.Exec(
			`INSERT INTO notifications.notifications (subscription_id, title, message) VALUES ($1, $2, $3)`,
			subscriptionID, "title", "message",
		)
		require.NoError(t, err)

		// Disabling (the replacement for deleting) must keep the history.
		_, err = db.Exec(`UPDATE notifications.subscriptions SET enabled = false WHERE subscription_id = $1`, subscriptionID)
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE notifications.topics SET enabled = false WHERE topic_id = $1`, topicID)
		require.NoError(t, err)

		var count int
		err = db.QueryRow(
			`SELECT count(*) FROM notifications.notifications WHERE subscription_id = $1`,
			subscriptionID,
		).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		_, err = db.Exec(`UPDATE notifications.subscriptions SET enabled = NULL WHERE subscription_id = $1`, subscriptionID)
		require.Error(t, err, "enabled must be NOT NULL")
	})

	t.Run("notifications.preferences carries a nullable notifications_last_seen", func(t *testing.T) {
		// Seeded without the column: a user who has never viewed their
		// notifications must read back NULL, not a backfilled default.
		_, err := db.Exec(
			`INSERT INTO notifications.preferences (user_id) VALUES ($1)
			 ON CONFLICT (user_id) DO UPDATE SET notifications_last_seen = NULL`,
			seededUserID,
		)
		require.NoError(t, err)

		var lastSeen sql.NullTime
		err = db.QueryRow(
			`SELECT notifications_last_seen FROM notifications.preferences WHERE user_id = $1`,
			seededUserID,
		).Scan(&lastSeen)
		require.NoError(t, err)
		require.False(t, lastSeen.Valid)

		// The upsert SetNotificationsLastSeen performs must store the value.
		written := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
		err = db.QueryRow(
			`INSERT INTO notifications.preferences (user_id, notifications_last_seen) VALUES ($1, $2)
			 ON CONFLICT (user_id) DO UPDATE SET notifications_last_seen = EXCLUDED.notifications_last_seen
			 RETURNING notifications_last_seen`,
			seededUserID, written,
		).Scan(&lastSeen)
		require.NoError(t, err)
		require.True(t, lastSeen.Valid)
		require.True(t, written.Equal(lastSeen.Time))
	})

	t.Run("notifications.subscriptions carries an optional, well-formed notification_channels", func(t *testing.T) {
		var topicID int
		err := db.QueryRow(
			`INSERT INTO notifications.topics (name) VALUES ($1) RETURNING topic_id`,
			"test-topic-channels",
		).Scan(&topicID)
		require.NoError(t, err)

		// No override by default: deliveries fall back to the user's preferences.
		var channels sql.NullString
		err = db.QueryRow(
			`INSERT INTO notifications.subscriptions (user_id, topic_id, context) VALUES ($1, $2, '{"n":1}')
			 RETURNING notification_channels`,
			seededUserID, topicID,
		).Scan(&channels)
		require.NoError(t, err)
		require.False(t, channels.Valid, "notification_channels must default to NULL")

		err = db.QueryRow(
			`INSERT INTO notifications.subscriptions (user_id, topic_id, context, notification_channels)
			 VALUES ($1, $2, '{"n":2}', '{"email":false,"push":true,"sms":false}')
			 RETURNING notification_channels`,
			seededUserID, topicID,
		).Scan(&channels)
		require.NoError(t, err)
		require.JSONEq(t, `{"email":false,"push":true,"sms":false}`, channels.String)

		for i, bad := range []string{
			`{"email":true,"push":false}`,                        // missing sms
			`{"email":true,"push":false,"sms":"yes"}`,            // not a boolean
			`{"email":true,"push":false,"sms":false,"fax":true}`, // unknown channel
			`[true,false,false]`,                                 // not an object
		} {
			_, err = db.Exec(
				`INSERT INTO notifications.subscriptions (user_id, topic_id, context, notification_channels)
				 VALUES ($1, $2, $3, $4)`,
				seededUserID, topicID, fmt.Sprintf(`{"bad":%d}`, i), bad,
			)
			require.Error(t, err, "CHECK constraint must reject %s", bad)
		}
	})

	t.Run("notifications.topics is seeded with the workspace publishing topics", func(t *testing.T) {
		workspace := `["organizationId"]`
		want := []struct{ name, required string }{
			{"DATASET_PUBLISHED", workspace},
			{"DATASET_EMBARGOED", workspace},
			{"DATASET_EMBARGO_RELEASED", workspace},
			{"DATASET_UNPUBLISHED", workspace},
			{"DATASET_REVISED", workspace},
		}
		for i, w := range want {
			var topicID int
			var enabled bool
			var required, additional string
			err := db.QueryRow(
				`SELECT topic_id, enabled, context->'required', context->'additionalProperties'
				 FROM notifications.topics WHERE name = $1`, w.name,
			).Scan(&topicID, &enabled, &required, &additional)
			require.NoError(t, err, w.name)
			require.Equal(t, i+1, topicID, "%s keeps its seeded position", w.name)
			require.True(t, enabled, w.name)
			require.JSONEq(t, w.required, required, w.name)
			require.Equal(t, "false", additional, w.name)
		}
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
