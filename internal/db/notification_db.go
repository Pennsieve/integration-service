package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/lib/pq"
)

// ErrTopicNotFound is returned when an operation references a topic id that
// does not exist in notifications.topics.
var ErrTopicNotFound = errors.New("topic not found")

// ErrTopicDisabled is returned when an operation would add or re-enable a
// subscription on a topic whose enabled flag is false.
var ErrTopicDisabled = errors.New("topic is disabled")

// ErrSubscriptionNotFound is returned when an operation references a
// subscription id that does not exist, or that belongs to another user.
var ErrSubscriptionNotFound = errors.New("subscription not found")

// ErrUserNotFound is returned when an operation references a user id that
// does not exist in pennsieve.users.
var ErrUserNotFound = errors.New("user not found")

// pqForeignKeyViolation is the error code Postgres reports when a foreign
// key constraint blocks an insert/update. See
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const pqForeignKeyViolation = "23503"

// GetTopics returns every topic, including disabled ones, so clients can
// still label notification history posted under a topic that has since been
// disabled. Only enabled topics accept new subscriptions.
func GetTopics(ctx context.Context) ([]models.Topic, error) {
	const q = `
		SELECT topic_id, name, description, enabled, created_at, context
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
		var topicContext []byte
		if err := rows.Scan(&t.TopicID, &t.Name, &description, &t.Enabled, &t.CreatedAt, &topicContext); err != nil {
			return nil, fmt.Errorf("get topics: %w", err)
		}
		t.Description = description.String
		t.Context = topicContext
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// GetUserSubscriptions returns every subscription belonging to userID, both
// enabled and disabled.
func GetUserSubscriptions(ctx context.Context, userID int64) ([]models.Subscription, error) {
	const q = `
		SELECT subscription_id, user_id, topic_id, context, enabled, created_at
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

// MatchSubscriptions returns every subscription to topicID whose context
// contains eventContext (context @> eventContext), the JSON object built by
// notification_matcher.ExtractContext. An empty object matches every
// subscription to the topic.
//
// Enabled flags are deliberately not checked here: matching decides which
// subscriptions an event is recorded against, and topics.enabled AND
// subscriptions.enabled only gate sending (see
// docs/notification-status-flags-scope.md), so both flags are returned for
// the caller to apply.
func MatchSubscriptions(ctx context.Context, topicID int64, eventContext []byte) ([]models.Subscription, error) {
	const q = `
		SELECT subscription_id, user_id, topic_id, context, enabled, created_at
		FROM notifications.subscriptions
		WHERE topic_id = $1 AND context @> $2::jsonb
		ORDER BY subscription_id`

	rows, err := dbPool.QueryContext(ctx, q, topicID, defaultJSON(eventContext))
	if err != nil {
		return nil, fmt.Errorf("match subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []models.Subscription
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("match subscriptions: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// CreateSubscription subscribes userID to topicID under the given context.
// A user can hold multiple subscriptions to the same topic as long as their
// contexts differ (e.g. following a topic for several datasets); calling it
// again with a context that already matches an existing row updates that
// row instead of creating a duplicate, since (user_id, topic_id, context) is
// unique. Subscribing again also re-enables that row if the user had
// disabled it, since subscriptions are disabled rather than deleted. The
// returned bool reports whether a new row was inserted (true)
// versus an existing row being updated (false), so callers can distinguish
// 201 Created from 200 OK.
//
// It also seeds a default notifications.preferences row for userID
// (email_enabled = true) if one doesn't already exist, so the delivery path
// always has a preferences row to read.
//
// The topic's enabled flag is read inside the same transaction as the
// insert, under FOR SHARE, so a concurrent disable of the topic blocks until
// this commits rather than slipping in between the check and the insert; a
// caller's earlier GetTopic read is not enough on its own, since the FK
// constraint alone is still satisfied by a disabled topic.
//
// Returns ErrTopicNotFound if topicID doesn't exist, and ErrTopicDisabled if
// it is disabled.
func CreateSubscription(ctx context.Context, userID, topicID int64, subscriptionContext []byte) (models.Subscription, bool, error) {
	tx, err := dbPool.BeginTx(ctx, nil)
	if err != nil {
		return models.Subscription{}, false, fmt.Errorf("create subscription: %w", err)
	}
	defer tx.Rollback()

	const topicQ = `
		SELECT enabled
		FROM notifications.topics
		WHERE topic_id = $1
		FOR SHARE`
	var topicEnabled bool
	if err := tx.QueryRowContext(ctx, topicQ, topicID).Scan(&topicEnabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Subscription{}, false, ErrTopicNotFound
		}
		return models.Subscription{}, false, fmt.Errorf("create subscription: lock topic: %w", err)
	}
	if !topicEnabled {
		return models.Subscription{}, false, ErrTopicDisabled
	}

	const subQ = `
		INSERT INTO notifications.subscriptions (user_id, topic_id, context)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, topic_id, context) DO UPDATE SET enabled = true
		RETURNING subscription_id, user_id, topic_id, context, enabled, created_at, (xmax = 0) AS inserted`

	row := tx.QueryRowContext(ctx, subQ, userID, topicID, defaultJSON(subscriptionContext))
	s, inserted, err := scanSubscriptionWithInserted(row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqForeignKeyViolation {
			return models.Subscription{}, false, ErrTopicNotFound
		}
		return models.Subscription{}, false, fmt.Errorf("create subscription: %w", err)
	}

	const prefQ = `
		INSERT INTO notifications.preferences (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, prefQ, userID); err != nil {
		return models.Subscription{}, false, fmt.Errorf("create subscription: seed preferences: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Subscription{}, false, fmt.Errorf("create subscription: %w", err)
	}
	return s, inserted, nil
}

// SetSubscriptionEnabled turns userID's subscription identified by
// subscriptionID on or off and returns the updated subscription. The update
// is scoped to userID so a user can never change another user's
// subscription by guessing its id; either way, a subscription the caller
// doesn't own reports ErrSubscriptionNotFound. Disabling only stops new
// notifications being delivered to the subscription: the row, and the
// notifications already posted to it, are kept.
func SetSubscriptionEnabled(ctx context.Context, subscriptionID, userID int64, enabled bool) (models.Subscription, error) {
	const q = `
		UPDATE notifications.subscriptions
		SET enabled = $3
		WHERE subscription_id = $1 AND user_id = $2
		RETURNING subscription_id, user_id, topic_id, context, enabled, created_at`

	s, err := scanSubscription(dbPool.QueryRowContext(ctx, q, subscriptionID, userID, enabled))
	if errors.Is(err, sql.ErrNoRows) {
		return models.Subscription{}, ErrSubscriptionNotFound
	}
	if err != nil {
		return models.Subscription{}, fmt.Errorf("set subscription enabled: %w", err)
	}
	return s, nil
}

// GetTopic returns the topic identified by topicID, including the context
// JSON Schema that subscriptions to it must satisfy. Returns
// ErrTopicNotFound if topicID doesn't exist.
func GetTopic(ctx context.Context, topicID int64) (models.Topic, error) {
	const q = `
		SELECT topic_id, name, description, enabled, created_at, context
		FROM notifications.topics
		WHERE topic_id = $1`

	var t models.Topic
	var description sql.NullString
	var topicContext []byte
	err := dbPool.QueryRowContext(ctx, q, topicID).Scan(&t.TopicID, &t.Name, &description, &t.Enabled, &t.CreatedAt, &topicContext)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Topic{}, ErrTopicNotFound
	}
	if err != nil {
		return models.Topic{}, fmt.Errorf("get topic: %w", err)
	}
	t.Description = description.String
	t.Context = topicContext
	return t, nil
}

// pqUndefinedTable is the error code Postgres reports when a query
// references a table that doesn't exist, including one qualified by a schema
// that doesn't exist, e.g. an organization schema for an organization id that
// was never provisioned.
const pqUndefinedTable = "42P01"

// DatasetExists reports whether datasetID exists, and is not being deleted,
// in organizationID's schema ("<organizationID>".datasets). An
// organizationID with no schema reports false rather than an error, since
// that just means the caller referenced an organization that doesn't exist.
func DatasetExists(ctx context.Context, organizationID, datasetID int64) (bool, error) {
	// The organization id is a schema name, which can't be bound as a query
	// parameter; it is an int64 formatted and quoted here, so it can't carry
	// SQL of its own.
	q := fmt.Sprintf(
		`SELECT EXISTS(SELECT 1 FROM %s.datasets WHERE id = $1 AND state <> 'DELETING')`,
		pq.QuoteIdentifier(strconv.FormatInt(organizationID, 10)))

	var exists bool
	if err := dbPool.QueryRowContext(ctx, q, datasetID).Scan(&exists); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqUndefinedTable {
			return false, nil
		}
		return false, fmt.Errorf("dataset exists: %w", err)
	}
	return exists, nil
}

// userNotificationsFrom is the FROM/WHERE clause shared by the page and
// count queries in GetUserNotifications, so the two can't drift apart and
// report a totalCount for a different set of rows than the page is cut from.
const userNotificationsFrom = `
		FROM notifications.notifications n
		JOIN notifications.subscriptions s ON s.subscription_id = n.subscription_id
		WHERE s.user_id = $1`

// GetUserNotifications returns one page of the notifications posted to any
// of userID's subscriptions, bounded by limit/offset, together with the total
// number of such notifications across all pages. Pages are ordered by
// created_at, newest first unless ascending is set, with notification_id as
// the tiebreaker so rows sharing a timestamp keep a stable order between
// pages. Notifications are scoped to a subscription rather than a user
// directly, so this joins through notifications.subscriptions to find the
// caller's own rows. Subscriptions the user has disabled are included too:
// disabling one stops new deliveries, but the history already posted to it
// stays readable.
//
// The total is counted by a separate query rather than COUNT(*) OVER () on
// the page query, since a page past the end has no rows to carry it.
func GetUserNotifications(ctx context.Context, userID int64, limit, offset int, ascending bool) ([]models.Notification, int, error) {
	var totalCount int
	if err := dbPool.QueryRowContext(ctx, `SELECT COUNT(*)`+userNotificationsFrom, userID).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count user notifications: %w", err)
	}

	// ORDER BY direction can't be a bind parameter, so pick between two
	// fixed clauses rather than formatting caller input into the SQL.
	orderBy := `
		ORDER BY n.created_at DESC, n.notification_id DESC`
	if ascending {
		orderBy = `
		ORDER BY n.created_at ASC, n.notification_id ASC`
	}
	q := `
		SELECT n.notification_id, n.subscription_id, s.topic_id, n.title, n.message, n.metadata, n.created_at` +
		userNotificationsFrom + orderBy + `
		LIMIT $2 OFFSET $3`

	rows, err := dbPool.QueryContext(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get user notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		var metadata []byte
		if err := rows.Scan(&n.NotificationID, &n.SubscriptionID, &n.TopicID, &n.Title, &n.Message, &metadata, &n.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("get user notifications: %w", err)
		}
		n.Metadata = metadata
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("get user notifications: %w", err)
	}
	return notifications, totalCount, nil
}

// subscriptionScanner abstracts over *sql.Row and *sql.Rows so
// scanSubscription can be shared by single-row and multi-row queries.
type subscriptionScanner interface {
	Scan(dest ...interface{}) error
}

func scanSubscription(row subscriptionScanner) (models.Subscription, error) {
	var s models.Subscription
	var context []byte
	if err := row.Scan(&s.SubscriptionID, &s.UserID, &s.TopicID, &context, &s.Enabled, &s.CreatedAt); err != nil {
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
	if err := row.Scan(&s.SubscriptionID, &s.UserID, &s.TopicID, &context, &s.Enabled, &s.CreatedAt, &inserted); err != nil {
		return models.Subscription{}, false, err
	}
	s.Context = context
	return s, inserted, nil
}

// defaultJSON turns an empty/nil JSON payload into an empty JSON object so
// the context column, which is NOT NULL, always gets a valid value.
func defaultJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

// defaultEmailEnabled and defaultPushEnabled mirror the email_enabled and
// push_enabled column defaults in migration
// 20260917090937_add_notifications_last_seen (DEFAULT true and DEFAULT
// false respectively). GetNotificationPreferences uses them for a user with
// no notifications.preferences row yet, since CreateSubscription's
// lazy-seed insert relies on those column defaults directly rather than on
// a value passed from Go. If a future migration changes either column
// default without updating these constants too, a user with no row would
// read back a stale default here while CreateSubscription would pick up the
// new one the moment they subscribe.
const (
	defaultEmailEnabled = true
	defaultPushEnabled  = false
)

// GetNotificationPreferences returns userID's email/push channel opt-ins and
// when they last viewed their notifications. A user with no
// notifications.preferences row yet (rows are seeded lazily by
// CreateSubscription) reads back the same defaults an inserted row would
// get: email enabled, push disabled, never seen. A missing row is therefore
// not an error.
func GetNotificationPreferences(ctx context.Context, userID int64) (*models.NotificationPreferences, error) {
	const q = `
		SELECT email_enabled, push_enabled, notifications_last_seen
		FROM notifications.preferences
		WHERE user_id = $1`

	prefs := &models.NotificationPreferences{LastSeen: models.LastSeen{UserID: userID}}
	var lastSeen sql.NullTime
	switch err := dbPool.QueryRowContext(ctx, q, userID).Scan(&prefs.EmailEnabled, &prefs.PushEnabled, &lastSeen); {
	case errors.Is(err, sql.ErrNoRows):
		prefs.EmailEnabled = defaultEmailEnabled
		prefs.PushEnabled = defaultPushEnabled
		return prefs, nil
	case err != nil:
		return nil, fmt.Errorf("get notification preferences: %w", err)
	}
	if lastSeen.Valid {
		prefs.NotificationsLastSeen = &lastSeen.Time
	}
	return prefs, nil
}

// SetNotificationPreferences upserts userID's email/push channel opt-ins and
// returns the full stored preferences, including whatever
// notifications_last_seen already held. The write is an upsert for the same
// reason SetNotificationsLastSeen's is: a user can change their notification
// settings before ever subscribing to a topic, i.e. before
// CreateSubscription has seeded a preferences row.
//
// Returns ErrUserNotFound if userID doesn't exist in pennsieve.users, which
// the preferences FK enforces.
func SetNotificationPreferences(ctx context.Context, userID int64, emailEnabled, pushEnabled bool) (*models.NotificationPreferences, error) {
	const q = `
		INSERT INTO notifications.preferences (user_id, email_enabled, push_enabled)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
			SET email_enabled = EXCLUDED.email_enabled,
			    push_enabled  = EXCLUDED.push_enabled
		RETURNING email_enabled, push_enabled, notifications_last_seen`

	prefs := &models.NotificationPreferences{LastSeen: models.LastSeen{UserID: userID}}
	var lastSeen sql.NullTime
	if err := dbPool.QueryRowContext(ctx, q, userID, emailEnabled, pushEnabled).Scan(&prefs.EmailEnabled, &prefs.PushEnabled, &lastSeen); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqForeignKeyViolation {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("set notification preferences: %w", err)
	}
	if lastSeen.Valid {
		prefs.NotificationsLastSeen = &lastSeen.Time
	}
	return prefs, nil
}

// SetNotificationsLastSeen records that userID viewed their notifications at
// lastSeen, and returns the stored value.
//
// The write is an upsert because notifications.preferences rows are seeded
// lazily by CreateSubscription: a user can open the notifications UI before
// ever subscribing to a topic, and that read should still be recordable. The
// inserted row therefore takes the same channel defaults CreateSubscription
// would have given it.
//
// Returns ErrUserNotFound if userID doesn't exist in pennsieve.users, which
// the preferences FK enforces.
func SetNotificationsLastSeen(ctx context.Context, userID int64, lastSeen time.Time) (time.Time, error) {
	const q = `
		INSERT INTO notifications.preferences (user_id, notifications_last_seen)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET notifications_last_seen = EXCLUDED.notifications_last_seen
		RETURNING notifications_last_seen`

	var stored time.Time
	if err := dbPool.QueryRowContext(ctx, q, userID, lastSeen.UTC()).Scan(&stored); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqForeignKeyViolation {
			return time.Time{}, ErrUserNotFound
		}
		return time.Time{}, fmt.Errorf("set notifications last seen: %w", err)
	}
	return stored, nil
}
