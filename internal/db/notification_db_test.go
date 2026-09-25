package db

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTopics(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT topic_id, name, description, enabled, created_at, context FROM notifications.topics")).
		WillReturnRows(sqlmock.NewRows([]string{"topic_id", "name", "description", "enabled", "created_at", "context"}).
			AddRow(int64(1), "datasets", "dataset events", true, now, []byte(`{"type":"object","properties":{"dataset_id":{"type":"integer"}}}`)).
			AddRow(int64(2), "billing", nil, false, now, nil))

	topics, err := GetTopics(context.Background())
	require.NoError(t, err)
	require.Len(t, topics, 2)
	assert.Equal(t, "datasets", topics[0].Name)
	assert.Equal(t, "dataset events", topics[0].Description)
	assert.Equal(t, "", topics[1].Description)
	assert.True(t, topics[0].Enabled)
	assert.False(t, topics[1].Enabled, "disabled topics are still listed")
	assert.JSONEq(t, `{"type":"object","properties":{"dataset_id":{"type":"integer"}}}`, string(topics[0].Context))
	assert.Nil(t, topics[1].Context)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserSubscriptions(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT subscription_id, user_id, topic_id, context, enabled, created_at FROM notifications.subscriptions")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(int64(1), int64(42), int64(2), []byte(`{"filter":"critical"}`), true, now).
			AddRow(int64(2), int64(42), int64(3), nil, false, now))

	subs, err := GetUserSubscriptions(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, subs, 2)
	assert.JSONEq(t, `{"filter":"critical"}`, string(subs[0].Context))
	assert.Nil(t, subs[1].Context)
	assert.True(t, subs[0].Enabled)
	assert.False(t, subs[1].Enabled, "disabled subscriptions are still listed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSubscription_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(7), []byte("{}")).
		WillReturnRows(sqlmock.NewRows(append(subscriptionColumns, "inserted")).
			AddRow(int64(9), int64(42), int64(7), []byte("{}"), true, now, true))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	sub, created, err := CreateSubscription(context.Background(), 42, 7, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(9), sub.SubscriptionID)
	assert.Equal(t, int64(42), sub.UserID)
	assert.Equal(t, int64(7), sub.TopicID)
	assert.True(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSubscription_Upsert_UpdatesExisting(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectBegin()
	// The conflict action must re-enable the row, so a user who disabled
	// this subscription gets it back by subscribing again.
	mock.ExpectQuery(regexp.QuoteMeta("ON CONFLICT (user_id, topic_id, context) DO UPDATE SET enabled = true")).
		WithArgs(int64(42), int64(7), []byte(`{"filter":"critical"}`)).
		WillReturnRows(sqlmock.NewRows(append(subscriptionColumns, "inserted")).
			AddRow(int64(9), int64(42), int64(7), []byte(`{"filter":"critical"}`), true, now, false))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	sub, created, err := CreateSubscription(context.Background(), 42, 7, []byte(`{"filter":"critical"}`))
	require.NoError(t, err)
	assert.Equal(t, int64(9), sub.SubscriptionID)
	assert.JSONEq(t, `{"filter":"critical"}`, string(sub.Context))
	assert.False(t, created, "re-subscribing with a context that already exists should report an update, not a new row")
	assert.True(t, sub.Enabled)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSubscription_TopicNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(999), []byte("{}")).
		WillReturnError(&pq.Error{Code: pqForeignKeyViolation})
	mock.ExpectRollback()

	_, _, err = CreateSubscription(context.Background(), 42, 999, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTopicNotFound))
	require.NoError(t, mock.ExpectationsWereMet())
}

var subscriptionColumns = []string{"subscription_id", "user_id", "topic_id", "context", "enabled", "created_at"}

// setSubscriptionEnabledQuery pins the full WHERE clause (not just the
// UPDATE prefix) so a regression that drops the "AND user_id = $2" scope
// would fail these tests.
const setSubscriptionEnabledQuery = "UPDATE notifications.subscriptions SET enabled = $3 WHERE subscription_id = $1 AND user_id = $2"

func TestSetSubscriptionEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		SetPoolForTest(mockDB)

		mock.ExpectQuery(regexp.QuoteMeta(setSubscriptionEnabledQuery)).
			WithArgs(int64(9), int64(42), enabled).
			WillReturnRows(sqlmock.NewRows(subscriptionColumns).
				AddRow(int64(9), int64(42), int64(7), []byte(`{"filter":"critical"}`), enabled, time.Now()))

		sub, err := SetSubscriptionEnabled(context.Background(), 9, 42, enabled)
		require.NoError(t, err)
		assert.Equal(t, int64(9), sub.SubscriptionID)
		assert.Equal(t, enabled, sub.Enabled)
		assert.JSONEq(t, `{"filter":"critical"}`, string(sub.Context))
		require.NoError(t, mock.ExpectationsWereMet())
		mockDB.Close()
	}
}

// Subscription 9 belongs to user 42; user 999 updating it must not match any
// row, since the update is scoped by user_id.
func TestSetSubscriptionEnabled_WrongUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta(setSubscriptionEnabledQuery)).
		WithArgs(int64(9), int64(999), false).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns))

	_, err = SetSubscriptionEnabled(context.Background(), 9, 999, false)
	assert.ErrorIs(t, err, ErrSubscriptionNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetSubscriptionEnabled_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta(setSubscriptionEnabledQuery)).
		WithArgs(int64(9), int64(42), false).
		WillReturnError(errors.New("connection reset"))

	_, err = SetSubscriptionEnabled(context.Background(), 9, 42, false)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSubscriptionNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserNotifications(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	// Pin the full WHERE clause so a regression that drops the
	// "s.user_id = $1" scope would fail this test. Deliberately no filter on
	// s.enabled: history under disabled subscriptions must still be returned.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT n.notification_id, n.subscription_id, s.topic_id, n.title, n.message, n.metadata, n.created_at FROM notifications.notifications n JOIN notifications.subscriptions s ON s.subscription_id = n.subscription_id WHERE s.user_id = $1 ORDER BY")).
		WithArgs(int64(42), 50, 0).
		WillReturnRows(sqlmock.NewRows([]string{"notification_id", "subscription_id", "topic_id", "title", "message", "metadata", "created_at"}).
			AddRow(int64(1), int64(20), int64(7), "Dataset published", "dataset 12 was published", []byte(`{"datasetId":12}`), now).
			AddRow(int64(2), int64(21), int64(8), "Dataset deleted", "dataset 5 was deleted", nil, now))

	notifications, err := GetUserNotifications(context.Background(), 42, 50, 0)
	require.NoError(t, err)
	require.Len(t, notifications, 2)
	assert.Equal(t, int64(7), notifications[0].TopicID)
	assert.Equal(t, int64(8), notifications[1].TopicID)
	assert.JSONEq(t, `{"datasetId":12}`, string(notifications[0].Metadata))
	assert.Nil(t, notifications[1].Metadata)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserNotifications_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.notifications n")).
		WithArgs(int64(42), 50, 0).
		WillReturnError(errors.New("connection reset"))

	_, err = GetUserNotifications(context.Background(), 42, 50, 0)
	require.Error(t, err)
}

func TestGetNotificationPreferences(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT email_enabled, push_enabled, notifications_last_seen FROM notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).
			AddRow(false, true, lastSeen))

	got, err := GetNotificationPreferences(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(42), got.UserID)
	assert.False(t, got.EmailEnabled)
	assert.True(t, got.PushEnabled)
	require.NotNil(t, got.NotificationsLastSeen)
	assert.True(t, got.NotificationsLastSeen.Equal(lastSeen))
	require.NoError(t, mock.ExpectationsWereMet())
}

// A user who has never viewed their notifications reads back a nil
// NotificationsLastSeen whether the column is NULL or the preferences row
// doesn't exist at all; a missing row also reads back the same defaults an
// inserted row would get.
func TestGetNotificationPreferences_NeverViewed(t *testing.T) {
	for _, tt := range []struct {
		name string
		rows *sqlmock.Rows
	}{
		{"null column", sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).AddRow(true, false, nil)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			SetPoolForTest(mockDB)

			mock.ExpectQuery(regexp.QuoteMeta("SELECT email_enabled, push_enabled, notifications_last_seen FROM notifications.preferences")).
				WithArgs(int64(42)).
				WillReturnRows(tt.rows)

			got, err := GetNotificationPreferences(context.Background(), 42)
			require.NoError(t, err)
			assert.Nil(t, got.NotificationsLastSeen)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetNotificationPreferences_NoRow_Defaults(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT email_enabled, push_enabled, notifications_last_seen FROM notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnError(sql.ErrNoRows)

	got, err := GetNotificationPreferences(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), got.UserID)
	assert.True(t, got.EmailEnabled, "a user with no preferences row yet should read back the schema default of email enabled")
	assert.False(t, got.PushEnabled)
	assert.Nil(t, got.NotificationsLastSeen)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetNotificationPreferences_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT email_enabled, push_enabled, notifications_last_seen FROM notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnError(errors.New("connection reset"))

	_, err = GetNotificationPreferences(context.Background(), 42)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetNotificationPreferences(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), false, true).
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).
			AddRow(false, true, lastSeen))

	got, err := SetNotificationPreferences(context.Background(), 42, false, true)
	require.NoError(t, err)
	assert.Equal(t, int64(42), got.UserID)
	assert.False(t, got.EmailEnabled)
	assert.True(t, got.PushEnabled)
	require.NotNil(t, got.NotificationsLastSeen)
	assert.True(t, got.NotificationsLastSeen.Equal(lastSeen))
	require.NoError(t, mock.ExpectationsWereMet())
}

// The preferences FK to pennsieve.users is what rejects an unknown user id,
// and that must surface as ErrUserNotFound rather than a generic failure.
func TestSetNotificationPreferences_UnknownUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(999), true, false).
		WillReturnError(&pq.Error{Code: pqForeignKeyViolation})

	_, err = SetNotificationPreferences(context.Background(), 999, true, false)
	assert.ErrorIs(t, err, ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetNotificationPreferences_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), true, false).
		WillReturnError(errors.New("connection reset"))

	_, err = SetNotificationPreferences(context.Background(), 42, true, false)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetNotificationsLastSeen(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	// Deliberately non-UTC: the stored value must be normalized to UTC.
	lastSeen := time.Date(2026, 9, 3, 10, 22, 0, 0, time.FixedZone("EDT", -4*60*60))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), lastSeen.UTC()).
		WillReturnRows(sqlmock.NewRows([]string{"notifications_last_seen"}).AddRow(lastSeen.UTC()))

	stored, err := SetNotificationsLastSeen(context.Background(), 42, lastSeen)
	require.NoError(t, err)
	assert.True(t, stored.Equal(lastSeen))
	require.NoError(t, mock.ExpectationsWereMet())
}

// The preferences FK to pennsieve.users is what rejects an unknown user id,
// and that must surface as ErrUserNotFound rather than a generic failure.
func TestSetNotificationsLastSeen_UnknownUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(999), lastSeen).
		WillReturnError(&pq.Error{Code: pqForeignKeyViolation})

	_, err = SetNotificationsLastSeen(context.Background(), 999, lastSeen)
	assert.ErrorIs(t, err, ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetNotificationsLastSeen_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), lastSeen).
		WillReturnError(errors.New("connection reset"))

	_, err = SetNotificationsLastSeen(context.Background(), 42, lastSeen)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopic(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.topics WHERE topic_id = $1")).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"topic_id", "name", "description", "enabled", "created_at", "context"}).
			AddRow(int64(4), "UPDATE_README", nil, false, time.Now(), []byte(`{"type":"object"}`)))

	topic, err := GetTopic(context.Background(), 4)
	require.NoError(t, err)
	assert.Equal(t, "UPDATE_README", topic.Name)
	assert.False(t, topic.Enabled)
	assert.JSONEq(t, `{"type":"object"}`, string(topic.Context))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopic_NotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.topics WHERE topic_id = $1")).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	_, err = GetTopic(context.Background(), 999)
	assert.ErrorIs(t, err, ErrTopicNotFound)
}

func TestDatasetExists(t *testing.T) {
	tests := []struct {
		name    string
		rows    *sqlmock.Rows
		err     error
		want    bool
		wantErr bool
	}{
		{name: "exists", rows: sqlmock.NewRows([]string{"exists"}).AddRow(true), want: true},
		{name: "missing dataset", rows: sqlmock.NewRows([]string{"exists"}).AddRow(false), want: false},
		{name: "missing organization schema", err: &pq.Error{Code: "42P01"}, want: false},
		{name: "db error", err: errors.New("connection reset"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			SetPoolForTest(mockDB)

			q := mock.ExpectQuery(regexp.QuoteMeta(`FROM "3".datasets WHERE id = $1 AND state <> 'DELETING'`)).WithArgs(int64(5))
			if tt.err != nil {
				q.WillReturnError(tt.err)
			} else {
				q.WillReturnRows(tt.rows)
			}

			got, err := DatasetExists(context.Background(), 3, 5)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
