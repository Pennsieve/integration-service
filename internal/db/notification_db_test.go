package db

import (
	"context"
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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT topic_id, name, description, created_at FROM notifications.topics")).
		WillReturnRows(sqlmock.NewRows([]string{"topic_id", "name", "description", "created_at"}).
			AddRow(int64(1), "datasets", "dataset events", now).
			AddRow(int64(2), "billing", nil, now))

	topics, err := GetTopics(context.Background())
	require.NoError(t, err)
	require.Len(t, topics, 2)
	assert.Equal(t, "datasets", topics[0].Name)
	assert.Equal(t, "dataset events", topics[0].Description)
	assert.Equal(t, "", topics[1].Description)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserSubscriptions(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT subscription_id, user_id, topic_id, context, created_at FROM notifications.subscriptions")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"subscription_id", "user_id", "topic_id", "context", "created_at"}).
			AddRow(int64(1), int64(42), int64(2), []byte(`{"filter":"critical"}`), now).
			AddRow(int64(2), int64(42), int64(3), nil, now))

	subs, err := GetUserSubscriptions(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, subs, 2)
	assert.JSONEq(t, `{"filter":"critical"}`, string(subs[0].Context))
	assert.Nil(t, subs[1].Context)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSubscription_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(7), nil).
		WillReturnRows(sqlmock.NewRows([]string{"subscription_id", "user_id", "topic_id", "context", "created_at", "inserted"}).
			AddRow(int64(9), int64(42), int64(7), nil, now, true))

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
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(7), []byte(`{"filter":"critical"}`)).
		WillReturnRows(sqlmock.NewRows([]string{"subscription_id", "user_id", "topic_id", "context", "created_at", "inserted"}).
			AddRow(int64(9), int64(42), int64(7), []byte(`{"filter":"critical"}`), now, false))

	sub, created, err := CreateSubscription(context.Background(), 42, 7, []byte(`{"filter":"critical"}`))
	require.NoError(t, err)
	assert.Equal(t, int64(9), sub.SubscriptionID)
	assert.JSONEq(t, `{"filter":"critical"}`, string(sub.Context))
	assert.False(t, created, "re-subscribing to an existing topic should report an update, not a new row")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSubscription_TopicNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(999), nil).
		WillReturnError(&pq.Error{Code: pqForeignKeyViolation})

	_, _, err = CreateSubscription(context.Background(), 42, 999, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTopicNotFound))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSubscription_Deleted(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM notifications.subscriptions")).
		WithArgs(int64(9), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	deleted, err := DeleteSubscription(context.Background(), 9, 42)
	require.NoError(t, err)
	assert.True(t, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSubscription_WrongUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	// subscription 9 belongs to user 42; user 999 attempting to delete it
	// must not match any row, since the query is scoped by user_id.
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM notifications.subscriptions")).
		WithArgs(int64(9), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	deleted, err := DeleteSubscription(context.Background(), 9, 999)
	require.NoError(t, err)
	assert.False(t, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSubscription_NotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM notifications.subscriptions")).
		WithArgs(int64(9), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	deleted, err := DeleteSubscription(context.Background(), 9, 42)
	require.NoError(t, err)
	assert.False(t, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTopicExists(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM notifications.topics WHERE topic_id = $1)")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := TopicExists(context.Background(), 7)
	require.NoError(t, err)
	assert.True(t, exists)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopicNotifications(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT n.notification_id, n.subscription_id, n.sender_id, n.title, n.message, n.metadata, n.created_at FROM notifications.notifications n JOIN notifications.subscriptions s ON s.subscription_id = n.subscription_id WHERE s.topic_id = $1")).
		WithArgs(int64(7), 50, 0).
		WillReturnRows(sqlmock.NewRows([]string{"notification_id", "subscription_id", "sender_id", "title", "message", "metadata", "created_at"}).
			AddRow(int64(1), int64(20), int64(3), "Dataset published", "dataset 12 was published", []byte(`{"datasetId":12}`), now).
			AddRow(int64(2), int64(21), int64(3), "Dataset deleted", "dataset 5 was deleted", nil, now))

	notifications, err := GetTopicNotifications(context.Background(), 7, 50, 0)
	require.NoError(t, err)
	require.Len(t, notifications, 2)
	assert.JSONEq(t, `{"datasetId":12}`, string(notifications[0].Metadata))
	assert.Nil(t, notifications[1].Metadata)
	require.NoError(t, mock.ExpectationsWereMet())
}
