package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/aws/aws-lambda-go/events"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func notifReq(method, rawPath string, pathParams map[string]string, userID string) events.APIGatewayV2HTTPRequest {
	req := events.APIGatewayV2HTTPRequest{
		RawPath:        rawPath,
		PathParameters: pathParams,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: method,
			},
		},
	}
	if userID != "" {
		req.RequestContext.Authorizer = &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			JWT: &events.APIGatewayV2HTTPRequestContextAuthorizerJWTDescription{
				Claims: map[string]string{"user_id": userID},
			},
		}
	}
	return req
}

func TestNotificationHandler_Unauthorized(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	resp, err := NotificationHandler(context.Background(), notifReq(http.MethodGet, "/notification/topics", nil, ""))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestNotificationHandler_GetTopics(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.topics")).
		WillReturnRows(sqlmock.NewRows([]string{"topic_id", "name", "description", "created_at"}).
			AddRow(int64(1), "datasets", "dataset events", now))

	resp, err := NotificationHandler(context.Background(), notifReq(http.MethodGet, "/notification/topics", nil, "42"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var topics []models.Topic
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &topics))
	require.Len(t, topics, 1)
	assert.Equal(t, "datasets", topics[0].Name)
}

func TestNotificationHandler_GetSubscriptions(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.subscriptions")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"subscription_id", "user_id", "topic_id", "context", "created_at"}))

	resp, err := NotificationHandler(context.Background(), notifReq(http.MethodGet, "/notification/subscriptions", nil, "42"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `[]`, resp.Body)
}

func TestNotificationHandler_Subscribe(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(7), nil).
		WillReturnRows(sqlmock.NewRows([]string{"subscription_id", "user_id", "topic_id", "context", "created_at"}).
			AddRow(int64(9), int64(42), int64(7), nil, now))

	req := notifReq(http.MethodPost, "/notification/subscriptions/7", map[string]string{"topicId": "7"}, "42")
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var sub models.Subscription
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &sub))
	assert.Equal(t, int64(9), sub.SubscriptionID)
}

func TestNotificationHandler_Subscribe_TopicNotFound(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(999), nil).
		WillReturnError(&pq.Error{Code: "23503"})

	req := notifReq(http.MethodPost, "/notification/subscriptions/999", map[string]string{"topicId": "999"}, "42")
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestNotificationHandler_Unsubscribe(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM notifications.subscriptions")).
		WithArgs(int64(9), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := notifReq(http.MethodDelete, "/notification/subscriptions/9", map[string]string{"subscriptionId": "9"}, "42")
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestNotificationHandler_Unsubscribe_NotFound(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM notifications.subscriptions")).
		WithArgs(int64(9), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req := notifReq(http.MethodDelete, "/notification/subscriptions/9", map[string]string{"subscriptionId": "9"}, "42")
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestNotificationHandler_GetTopicNotifications(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM notifications.topics WHERE topic_id = $1)")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.notifications")).
		WithArgs(int64(7), defaultNotificationsLimit, 0).
		WillReturnRows(sqlmock.NewRows([]string{"notification_id", "topic_id", "sender_id", "title", "message", "metadata", "created_at"}).
			AddRow(int64(1), int64(7), int64(3), "Dataset published", "dataset 12 was published", nil, now))

	req := notifReq(http.MethodGet, "/notification/7/notifications", map[string]string{"topicId": "7"}, "42")
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var notifications []models.Notification
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &notifications))
	require.Len(t, notifications, 1)
	assert.Equal(t, "Dataset published", notifications[0].Title)
}

func TestNotificationHandler_GetTopicNotifications_TopicNotFound(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM notifications.topics WHERE topic_id = $1)")).
		WithArgs(int64(999)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	req := notifReq(http.MethodGet, "/notification/999/notifications", map[string]string{"topicId": "999"}, "42")
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestNotificationHandler_NotFoundRoute(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	req := notifReq(http.MethodGet, "/notification/unknown", nil, "42")
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
