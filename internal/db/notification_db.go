package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/lib/pq"
)

// ErrTopicNotFound is returned when an operation references a topic id that
// does not exist in notifications.topics.
var ErrTopicNotFound = errors.New("topic not found")

// pqForeignKeyViolation is the error code Postgres reports when a foreign
// key constraint blocks an insert/update. See
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const pqForeignKeyViolation = "23503"

// GetTopics returns every topic a user may subscribe to.
func GetTopics(ctx context.Context) ([]models.Topic, error) {
	const q = `
		SELECT topic_id, name, description, created_at
		FROM notifications.topics
		ORDER BY name`

	rows, err := dbPool.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get topics: %w", err)
	}
	defer rows.Close()

	var topics []models.Topic
	for rows.Next() {
		var t models.Topic
		var description sql.NullString
		if err := rows.Scan(&t.TopicID, &t.Name, &description, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("get topics: %w", err)
		}
		t.Description = description.String
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// GetUserSubscriptions returns every subscription belonging to userID.
func GetUserSubscriptions(ctx context.Context, userID int64) ([]models.Subscription, error) {
	const q = `
		SELECT subscription_id, user_id, topic_id, context, created_at
		FROM notifications.subscriptions
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := dbPool.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get user subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []models.Subscription
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("get user subscriptions: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// CreateSubscription subscribes userID to topicID. Calling it again for a
// topic the user is already subscribed to updates the stored context rather
// than creating a duplicate row, since (user_id, topic_id) is unique. The
// returned bool reports whether a new row was inserted (true) versus an
// existing row's context being updated (false), so callers can distinguish
// 201 Created from 200 OK.
// Returns ErrTopicNotFound if topicID doesn't exist.
func CreateSubscription(ctx context.Context, userID, topicID int64, subscriptionContext []byte) (models.Subscription, bool, error) {
	const q = `
		INSERT INTO notifications.subscriptions (user_id, topic_id, context)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, topic_id) DO UPDATE SET context = EXCLUDED.context
		RETURNING subscription_id, user_id, topic_id, context, created_at, (xmax = 0) AS inserted`

	row := dbPool.QueryRowContext(ctx, q, userID, topicID, nullableJSON(subscriptionContext))
	s, inserted, err := scanSubscriptionWithInserted(row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqForeignKeyViolation {
			return models.Subscription{}, false, ErrTopicNotFound
		}
		return models.Subscription{}, false, fmt.Errorf("create subscription: %w", err)
	}
	return s, inserted, nil
}

// DeleteSubscription removes userID's subscription identified by
// subscriptionID. The delete is scoped to userID so a user can never delete
// another user's subscription by guessing its id. Returns false if no
// matching subscription was found.
func DeleteSubscription(ctx context.Context, subscriptionID, userID int64) (bool, error) {
	const q = `
		DELETE FROM notifications.subscriptions
		WHERE subscription_id = $1 AND user_id = $2`

	res, err := dbPool.ExecContext(ctx, q, subscriptionID, userID)
	if err != nil {
		return false, fmt.Errorf("delete subscription: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete subscription: %w", err)
	}
	return n > 0, nil
}

// TopicExists reports whether topicID exists in notifications.topics.
func TopicExists(ctx context.Context, topicID int64) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM notifications.topics WHERE topic_id = $1)`

	var exists bool
	if err := dbPool.QueryRowContext(ctx, q, topicID).Scan(&exists); err != nil {
		return false, fmt.Errorf("topic exists: %w", err)
	}
	return exists, nil
}

// GetTopicNotifications returns notifications posted to topicID, newest
// first, bounded by limit/offset. Notifications are scoped to a subscription
// rather than a topic directly, so this joins through
// notifications.subscriptions to find rows for that topic.
func GetTopicNotifications(ctx context.Context, topicID int64, limit, offset int) ([]models.Notification, error) {
	const q = `
		SELECT n.notification_id, n.subscription_id, n.sender_id, n.title, n.message, n.metadata, n.created_at
		FROM notifications.notifications n
		JOIN notifications.subscriptions s ON s.subscription_id = n.subscription_id
		WHERE s.topic_id = $1
		ORDER BY n.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := dbPool.QueryContext(ctx, q, topicID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get topic notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		var metadata []byte
		if err := rows.Scan(&n.NotificationID, &n.SubscriptionID, &n.SenderID, &n.Title, &n.Message, &metadata, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("get topic notifications: %w", err)
		}
		n.Metadata = metadata
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

// subscriptionScanner abstracts over *sql.Row and *sql.Rows so
// scanSubscription can be shared by single-row and multi-row queries.
type subscriptionScanner interface {
	Scan(dest ...interface{}) error
}

func scanSubscription(row subscriptionScanner) (models.Subscription, error) {
	var s models.Subscription
	var context []byte
	if err := row.Scan(&s.SubscriptionID, &s.UserID, &s.TopicID, &context, &s.CreatedAt); err != nil {
		return models.Subscription{}, err
	}
	s.Context = context
	return s, nil
}

// scanSubscriptionWithInserted is like scanSubscription but also reads the
// "(xmax = 0) AS inserted" column CreateSubscription's upsert appends to its
// RETURNING clause: a row's xmax is unset (0) only when it was inserted by
// this statement, and set to the current transaction when an existing row
// was updated via ON CONFLICT DO UPDATE.
func scanSubscriptionWithInserted(row subscriptionScanner) (models.Subscription, bool, error) {
	var s models.Subscription
	var context []byte
	var inserted bool
	if err := row.Scan(&s.SubscriptionID, &s.UserID, &s.TopicID, &context, &s.CreatedAt, &inserted); err != nil {
		return models.Subscription{}, false, err
	}
	s.Context = context
	return s, inserted, nil
}

// nullableJSON turns an empty/nil JSON payload into a SQL NULL so the
// context column stores NULL instead of an empty byte slice.
func nullableJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}
