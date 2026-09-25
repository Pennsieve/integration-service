package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

// unauthedNotifReq builds the request API Gateway sends for routeKey (e.g.
// routeSubscribe) with the given path parameters. RawPath is the route's
// path template with those parameters substituted, under the API mapping's
// /notification base path.
func unauthedNotifReq(routeKey string, pathParams map[string]string) events.APIGatewayV2HTTPRequest {
	method, template, _ := strings.Cut(routeKey, " ")
	rawPath := template
	for key, value := range pathParams {
		rawPath = strings.ReplaceAll(rawPath, "{"+key+"}", value)
	}
	return events.APIGatewayV2HTTPRequest{
		RouteKey:       routeKey,
		RawPath:        "/notification" + rawPath,
		PathParameters: pathParams,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: method,
			},
		},
	}
}

func authedNotifReq(routeKey string, pathParams map[string]string, userID int64) events.APIGatewayV2HTTPRequest {
	req := unauthedNotifReq(routeKey, pathParams)
	req.RequestContext.Authorizer = &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
		Lambda: map[string]interface{}{
			"user_claim": map[string]interface{}{
				"Id":           float64(userID),
				"NodeId":       "N:user:test",
				"IsSuperAdmin": false,
			},
		},
	}
	return req
}

func TestNotificationHandler_Unauthorized(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	resp, err := NotificationHandler(context.Background(), unauthedNotifReq(routeGetTopics, nil))
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
		WillReturnRows(sqlmock.NewRows([]string{"topic_id", "name", "description", "enabled", "created_at", "context"}).
			AddRow(int64(1), "datasets", "dataset events", true, now, []byte(`{"type":"object"}`)))

	resp, err := NotificationHandler(context.Background(), authedNotifReq(routeGetTopics, nil, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var topics []models.Topic
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &topics))
	require.Len(t, topics, 1)
	assert.Equal(t, "datasets", topics[0].Name)
	assert.True(t, topics[0].Enabled)
	assert.JSONEq(t, `{"type":"object"}`, string(topics[0].Context))
}

func TestNotificationHandler_GetSubscriptions(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.subscriptions")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns))

	resp, err := NotificationHandler(context.Background(), authedNotifReq(routeGetSubscriptions, nil, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `[]`, resp.Body)
}

func TestNotificationHandler_GetSubscriptions_DBError(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.subscriptions")).
		WithArgs(int64(42)).
		WillReturnError(errors.New("connection reset"))

	resp, err := NotificationHandler(context.Background(), authedNotifReq(routeGetSubscriptions, nil, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestNotificationHandler_MissingUserClaim(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	req := unauthedNotifReq(routeGetTopics, nil)
	req.RequestContext.Authorizer = &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
		Lambda: map[string]interface{}{},
	}
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// updateReadmeContext is the context JSON Schema of the UPDATE_README topic.
const updateReadmeContext = `{
	"$schema": "http://json-schema.org",
	"properties": {
		"datasetId": {"type": "integer"},
		"organizationId": {"type": "integer"}
	},
	"required": ["organizationId", "datasetId"],
	"title": "UpdateReadmeTopicConfiguration",
	"type": "object"
}`

var (
	topicColumns        = []string{"topic_id", "name", "description", "enabled", "created_at", "context"}
	subscriptionColumns = []string{"subscription_id", "user_id", "topic_id", "context", "enabled", "created_at"}
)

// expectGetTopic registers the topic lookup handleSubscribe performs before
// validating the body. A nil topicContext is a topic with no context schema.
func expectGetTopic(mock sqlmock.Sqlmock, topicID int64, topicContext []byte) {
	expectGetTopicEnabled(mock, topicID, topicContext, true)
}

func expectGetTopicEnabled(mock sqlmock.Sqlmock, topicID int64, topicContext []byte, enabled bool) {
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.topics")).
		WithArgs(topicID).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(topicID, "UPDATE_README", "dataset readme updated", enabled, time.Now(), topicContext))
}

// expectCreateSubscription registers CreateSubscription's transaction,
// storing storedContext.
func expectCreateSubscription(mock sqlmock.Sqlmock, userID, topicID int64, storedContext []byte, inserted bool) {
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(userID, topicID, storedContext).
		WillReturnRows(sqlmock.NewRows(append(subscriptionColumns, "inserted")).
			AddRow(int64(9), userID, topicID, storedContext, true, time.Now(), inserted))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func newSubscribeMock(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mockDB.Close() })
	db.SetPoolForTest(mockDB)
	return mock
}

func TestNotificationHandler_Subscribe(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 7, nil)
	expectCreateSubscription(mock, 42, 7, []byte("{}"), true)

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var sub models.Subscription
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &sub))
	assert.Equal(t, int64(9), sub.SubscriptionID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestNotificationHandler_Subscribe_EmptyOrNullBody checks that a missing,
// blank, or JSON null body is stored as {} on a topic with no context
// schema, rather than being rejected as not a JSON object.
func TestNotificationHandler_Subscribe_EmptyOrNullBody(t *testing.T) {
	for name, body := range map[string]string{
		"empty":             ``,
		"whitespace":        " \n\t",
		"null":              `null`,
		"whitespace-padded": " null\n",
	} {
		t.Run(name, func(t *testing.T) {
			mock := newSubscribeMock(t)
			expectGetTopic(mock, 7, nil)
			expectCreateSubscription(mock, 42, 7, []byte("{}"), true)

			req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
			req.Body = body
			resp, err := NotificationHandler(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode, resp.Body)

			var sub models.Subscription
			require.NoError(t, json.Unmarshal([]byte(resp.Body), &sub))
			assert.JSONEq(t, `{}`, string(sub.Context))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestNotificationHandler_Subscribe_Upsert(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 7, nil)
	expectCreateSubscription(mock, 42, 7, []byte("{}"), false)

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "re-subscribing to an already-subscribed topic should return 200, not 201")

	var sub models.Subscription
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &sub))
	assert.Equal(t, int64(9), sub.SubscriptionID)
	assert.True(t, sub.Enabled, "re-subscribing should re-enable a disabled subscription")
}

// TestNotificationHandler_Subscribe_UpsertReEnables pins the upsert's
// conflict action: re-subscribing must turn a disabled subscription back on.
func TestNotificationHandler_Subscribe_UpsertReEnables(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 7, nil)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("ON CONFLICT (user_id, topic_id, context) DO UPDATE SET enabled = true")).
		WithArgs(int64(42), int64(7), []byte("{}")).
		WillReturnRows(sqlmock.NewRows(append(subscriptionColumns, "inserted")).
			AddRow(int64(9), int64(42), int64(7), []byte("{}"), true, time.Now(), false))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := NotificationHandler(context.Background(), authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, resp.Body)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A disabled topic keeps its existing subscriptions and history but accepts
// no new subscriptions.
func TestNotificationHandler_Subscribe_DisabledTopic(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopicEnabled(mock, 7, nil, false)

	resp, err := NotificationHandler(context.Background(), authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, resp.Body, "topic is disabled")
	assert.NoError(t, mock.ExpectationsWereMet(), "no subscription should be stored")
}

func TestNotificationHandler_Subscribe_Base64Body(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 7, nil)
	expectCreateSubscription(mock, 42, 7, []byte(`{"filter":"critical"}`), true)

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	req.Body = base64.StdEncoding.EncodeToString([]byte(`{"filter":"critical"}`))
	req.IsBase64Encoded = true
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var sub models.Subscription
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &sub))
	assert.JSONEq(t, `{"filter":"critical"}`, string(sub.Context))
}

func TestNotificationHandler_Subscribe_ValidTopicContext(t *testing.T) {
	mock := newSubscribeMock(t)
	body := []byte(`{"organizationId": 1, "datasetId": 5}`)
	expectGetTopic(mock, 4, []byte(updateReadmeContext))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "1".datasets`)).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	expectCreateSubscription(mock, 42, 4, body, true)

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "4"}, 42)
	req.Body = string(body)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, resp.Body)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_Subscribe_TopicContextMismatch(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{"missing required property", `{"organizationId": 1}`, "datasetId"},
		{"wrong property type", `{"organizationId": 1, "datasetId": "five"}`, "datasetId"},
		{"empty body", ``, "missing properties"},
		{"null body", `null`, "missing properties"},
		{"whitespace-padded null body", " null\n", "missing properties"},
		{"not an object", `[1, 2]`, "JSON object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newSubscribeMock(t)
			expectGetTopic(mock, 4, []byte(updateReadmeContext))

			req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "4"}, 42)
			req.Body = tt.body
			resp, err := NotificationHandler(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errBody models.NotificationErrorResponse
			require.NoError(t, json.Unmarshal([]byte(resp.Body), &errBody))
			assert.Contains(t, errBody.Message, tt.wantMessage)
			assert.NoError(t, mock.ExpectationsWereMet(), "no subscription should be stored")
		})
	}
}

func TestNotificationHandler_Subscribe_DatasetNotFound(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 4, []byte(updateReadmeContext))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "1".datasets`)).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "4"}, 42)
	req.Body = `{"organizationId": 1, "datasetId": 5}`
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, resp.Body, "dataset not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_Subscribe_OrganizationNotFound(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 4, []byte(updateReadmeContext))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "999".datasets`)).
		WithArgs(int64(5)).
		WillReturnError(&pq.Error{Code: "42P01"})

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "4"}, 42)
	req.Body = `{"organizationId": 999, "datasetId": 5}`
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, resp.Body, "dataset not found")
}

func TestNotificationHandler_Subscribe_InvalidTopicSchema(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 4, []byte(`{"type": 12}`))

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "4"}, 42)
	req.Body = `{}`
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestNotificationHandler_Subscribe_InvalidBase64(t *testing.T) {
	newSubscribeMock(t)

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	req.Body = "not-valid-base64!!"
	req.IsBase64Encoded = true
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNotificationHandler_Subscribe_InvalidJSON(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 7, nil)

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	req.Body = "{not json"
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNotificationHandler_Subscribe_InvalidTopicID(t *testing.T) {
	newSubscribeMock(t)

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "abc"}, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNotificationHandler_Subscribe_DBError(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 7, nil)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(7), []byte("{}")).
		WillReturnError(errors.New("connection reset"))
	mock.ExpectRollback()

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestNotificationHandler_Subscribe_TopicNotFound(t *testing.T) {
	mock := newSubscribeMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.topics")).
		WithArgs(int64(999)).
		WillReturnRows(sqlmock.NewRows(topicColumns))

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "999"}, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet(), "no subscription should be stored")
}

// TestNotificationHandler_Subscribe_TopicDeletedBeforeInsert covers the
// window between the topic lookup and the insert: the topic exists when
// looked up but is gone by the time the subscription is inserted, so the
// insert hits the topic foreign key. That must still be a 404, not a 500.
func TestNotificationHandler_Subscribe_TopicDeletedBeforeInsert(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 7, nil)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.subscriptions")).
		WithArgs(int64(42), int64(7), []byte("{}")).
		WillReturnError(&pq.Error{Code: "23503"})
	mock.ExpectRollback()

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, resp.Body, "topic not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_Subscribe_GetTopicDBError(t *testing.T) {
	mock := newSubscribeMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.topics")).
		WithArgs(int64(7)).
		WillReturnError(errors.New("connection reset"))

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet(), "no subscription should be stored")
}

func TestNotificationHandler_Subscribe_DatasetLookupDBError(t *testing.T) {
	mock := newSubscribeMock(t)
	expectGetTopic(mock, 4, []byte(updateReadmeContext))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "1".datasets`)).
		WithArgs(int64(5)).
		WillReturnError(errors.New("connection reset"))

	req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "4"}, 42)
	req.Body = `{"organizationId": 1, "datasetId": 5}`
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet(), "no subscription should be stored")
}

// TestNotificationHandler_Subscribe_InvalidDatasetReference covers dataset
// references that the topic's schema doesn't rule out (here, a topic with no
// context) but that can't name a real dataset.
func TestNotificationHandler_Subscribe_InvalidDatasetReference(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{"dataset without organization", `{"datasetId": 5}`, "datasetId requires organizationId"},
		{"zero organization", `{"organizationId": 0, "datasetId": 5}`, "organizationId must be a positive integer"},
		{"non-numeric organization", `{"organizationId": "1", "datasetId": 5}`, "organizationId must be a positive integer"},
		{"negative dataset", `{"organizationId": 1, "datasetId": -5}`, "datasetId must be a positive integer"},
		{"fractional dataset", `{"organizationId": 1, "datasetId": 5.5}`, "datasetId must be a positive integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newSubscribeMock(t)
			expectGetTopic(mock, 7, nil)

			req := authedNotifReq(routeSubscribe, map[string]string{"topicId": "7"}, 42)
			req.Body = tt.body
			resp, err := NotificationHandler(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, resp.Body, tt.wantMessage)
			assert.NoError(t, mock.ExpectationsWereMet(), "no subscription should be stored")
		})
	}
}

func expectSetSubscriptionEnabled(mock sqlmock.Sqlmock, subscriptionID, userID int64, enabled bool) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(regexp.QuoteMeta("UPDATE notifications.subscriptions SET enabled = $3 WHERE subscription_id = $1 AND user_id = $2")).
		WithArgs(subscriptionID, userID, enabled)
}

func TestNotificationHandler_UpdateSubscription(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(strconv.FormatBool(enabled), func(t *testing.T) {
			mock := newSubscribeMock(t)
			expectSetSubscriptionEnabled(mock, 9, 42, enabled).
				WillReturnRows(sqlmock.NewRows(subscriptionColumns).
					AddRow(int64(9), int64(42), int64(7), []byte("{}"), enabled, time.Now()))

			resp, err := NotificationHandler(context.Background(),
				notifReqWithBody(routeUpdateSubscription, map[string]string{"subscriptionId": "9"}, 42,
					`{"enabled":`+strconv.FormatBool(enabled)+`}`))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode, resp.Body)

			var sub models.Subscription
			require.NoError(t, json.Unmarshal([]byte(resp.Body), &sub))
			assert.Equal(t, int64(9), sub.SubscriptionID)
			assert.Equal(t, enabled, sub.Enabled)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestNotificationHandler_UpdateSubscription_Base64Body(t *testing.T) {
	mock := newSubscribeMock(t)
	expectSetSubscriptionEnabled(mock, 9, 42, false).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(int64(9), int64(42), int64(7), []byte("{}"), false, time.Now()))

	req := notifReqWithBody(routeUpdateSubscription, map[string]string{"subscriptionId": "9"}, 42,
		base64.StdEncoding.EncodeToString([]byte(`{"enabled":false}`)))
	req.IsBase64Encoded = true
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_UpdateSubscription_NotFound(t *testing.T) {
	mock := newSubscribeMock(t)
	expectSetSubscriptionEnabled(mock, 9, 42, false).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns))

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateSubscription, map[string]string{"subscriptionId": "9"}, 42, `{"enabled":false}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Subscription 9 belongs to user 42. A different authenticated caller (999)
// must not be able to change it: the update is scoped by user_id (pinned by
// expectSetSubscriptionEnabled's full WHERE clause), so it matches no row
// and the handler reports not-found rather than leaking whether the
// subscription exists for someone else.
func TestNotificationHandler_UpdateSubscription_WrongUser(t *testing.T) {
	mock := newSubscribeMock(t)
	expectSetSubscriptionEnabled(mock, 9, 999, false).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns))

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateSubscription, map[string]string{"subscriptionId": "9"}, 999, `{"enabled":false}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_UpdateSubscription_BadRequest(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"not json", `not json`},
		{"empty body", ``},
		{"missing field", `{}`},
		{"explicit null", `{"enabled":null}`},
		{"wrong type", `{"enabled":"no"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mock := newSubscribeMock(t)

			resp, err := NotificationHandler(context.Background(),
				notifReqWithBody(routeUpdateSubscription, map[string]string{"subscriptionId": "9"}, 42, tt.body))
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestNotificationHandler_UpdateSubscription_InvalidSubscriptionID(t *testing.T) {
	mock := newSubscribeMock(t)

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateSubscription, map[string]string{"subscriptionId": "abc"}, 42, `{"enabled":false}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_UpdateSubscription_DBError(t *testing.T) {
	mock := newSubscribeMock(t)
	expectSetSubscriptionEnabled(mock, 9, 42, false).
		WillReturnError(errors.New("connection reset"))

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateSubscription, map[string]string{"subscriptionId": "9"}, 42, `{"enabled":false}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// The DELETE and by-topic routes were removed in favor of disabling
// subscriptions and GET /messages; neither must still be served.
func TestNotificationHandler_RemovedRoutes(t *testing.T) {
	for _, routeKey := range []string{
		"DELETE /subscriptions/{subscriptionId}",
		"GET /{topicId}/notifications",
	} {
		t.Run(routeKey, func(t *testing.T) {
			mock := newSubscribeMock(t)
			resp, err := NotificationHandler(context.Background(), authedNotifReq(routeKey, nil, 42))
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

var notificationColumns = []string{"notification_id", "subscription_id", "topic_id", "title", "message", "metadata", "created_at"}

func TestNotificationHandler_GetMessages(t *testing.T) {
	mock := newSubscribeMock(t)
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.notifications n JOIN notifications.subscriptions s ON s.subscription_id = n.subscription_id WHERE s.user_id = $1")).
		WithArgs(int64(42), defaultNotificationsLimit, 0).
		WillReturnRows(sqlmock.NewRows(notificationColumns).
			AddRow(int64(2), int64(21), int64(8), "Readme updated", "dataset 5 readme changed", []byte(`{"datasetId":5}`), now).
			AddRow(int64(1), int64(20), int64(7), "Dataset published", "dataset 12 was published", nil, now.Add(-time.Hour)))

	resp, err := NotificationHandler(context.Background(), authedNotifReq(routeGetMessages, nil, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var notifications []models.Notification
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &notifications))
	require.Len(t, notifications, 2)
	assert.Equal(t, "Readme updated", notifications[0].Title)
	assert.Equal(t, int64(8), notifications[0].TopicID)
	assert.JSONEq(t, `{"datasetId":5}`, string(notifications[0].Metadata))
	assert.Equal(t, int64(7), notifications[1].TopicID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_GetMessages_Pagination(t *testing.T) {
	mock := newSubscribeMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.notifications n")).
		WithArgs(int64(42), 10, 20).
		WillReturnRows(sqlmock.NewRows(notificationColumns))

	req := authedNotifReq(routeGetMessages, nil, 42)
	req.QueryStringParameters = map[string]string{"limit": "10", "offset": "20"}
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `[]`, resp.Body)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_GetMessages_DBError(t *testing.T) {
	mock := newSubscribeMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.notifications n")).
		WithArgs(int64(42), defaultNotificationsLimit, 0).
		WillReturnError(errors.New("connection reset"))

	resp, err := NotificationHandler(context.Background(), authedNotifReq(routeGetMessages, nil, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestParsePagination(t *testing.T) {
	cases := []struct {
		name       string
		params     map[string]string
		wantLimit  int
		wantOffset int
	}{
		{"defaults when absent", nil, defaultNotificationsLimit, 0},
		{"valid values pass through", map[string]string{"limit": "10", "offset": "5"}, 10, 5},
		{"limit over max falls back to default", map[string]string{"limit": "99999"}, defaultNotificationsLimit, 0},
		{"limit at max is kept", map[string]string{"limit": "200"}, maxNotificationsLimit, 0},
		{"zero limit falls back to default", map[string]string{"limit": "0"}, defaultNotificationsLimit, 0},
		{"negative limit falls back to default", map[string]string{"limit": "-5"}, defaultNotificationsLimit, 0},
		{"non-numeric limit falls back to default", map[string]string{"limit": "abc"}, defaultNotificationsLimit, 0},
		{"negative offset falls back to zero", map[string]string{"offset": "-1"}, defaultNotificationsLimit, 0},
		{"non-numeric offset falls back to zero", map[string]string{"offset": "abc"}, defaultNotificationsLimit, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			limit, offset := parsePagination(c.params)
			assert.Equal(t, c.wantLimit, limit)
			assert.Equal(t, c.wantOffset, offset)
		})
	}
}

func TestNotificationHandler_NotFoundRoute(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	req := authedNotifReq("GET /unknown", nil, 42)
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// notifReqWithBody is authedNotifReq plus a JSON body, for the POST/PATCH routes.
func notifReqWithBody(routeKey string, pathParams map[string]string, userID int64, body string) events.APIGatewayV2HTTPRequest {
	req := authedNotifReq(routeKey, pathParams, userID)
	req.Body = body
	return req
}

func TestNotificationHandler_GetNotificationPreferences(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT email_enabled, push_enabled, notifications_last_seen FROM notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).
			AddRow(true, false, lastSeen))

	resp, err := NotificationHandler(context.Background(),
		authedNotifReq(routeGetUserPreferences, map[string]string{"userId": "42"}, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var got models.NotificationPreferences
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &got))
	assert.Equal(t, int64(42), got.UserID)
	assert.True(t, got.EmailEnabled)
	assert.False(t, got.PushEnabled)
	require.NotNil(t, got.NotificationsLastSeen)
	assert.True(t, got.NotificationsLastSeen.Equal(lastSeen))
}

// A user who has never viewed their notifications must serialize the field
// as an explicit null, not omit it and not report the Go zero time.
func TestNotificationHandler_GetNotificationPreferences_NeverViewed(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT email_enabled, push_enabled, notifications_last_seen FROM notifications.preferences")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).
			AddRow(true, false, nil))

	resp, err := NotificationHandler(context.Background(),
		authedNotifReq(routeGetUserPreferences, map[string]string{"userId": "42"}, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"userId":42,"emailEnabled":true,"pushEnabled":false,"notificationsLastSeen":null}`, resp.Body)
}

func TestNotificationHandler_SetNotificationPreferences(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), false, true).
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).
			AddRow(false, true, nil))

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeSetUserPreferences, map[string]string{"userId": "42"}, 42,
			`{"emailEnabled":false,"pushEnabled":true}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"userId":42,"emailEnabled":false,"pushEnabled":true,"notificationsLastSeen":null}`, resp.Body)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_SetNotificationPreferences_Base64Body(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), true, true).
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).
			AddRow(true, true, nil))

	req := notifReqWithBody(routeSetUserPreferences, map[string]string{"userId": "42"}, 42,
		base64.StdEncoding.EncodeToString([]byte(`{"emailEnabled":true,"pushEnabled":true}`)))
	req.IsBase64Encoded = true

	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

// The core auth requirement: one user may not write another's preferences.
// No DB call should be attempted, which mock.ExpectationsWereMet asserts by
// way of no expectations having been registered.
func TestNotificationHandler_SetNotificationPreferences_OtherUserForbidden(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeSetUserPreferences, map[string]string{"userId": "99"}, 42,
			`{"emailEnabled":true,"pushEnabled":true}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_GetNotificationPreferences_OtherUserForbidden(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	resp, err := NotificationHandler(context.Background(),
		authedNotifReq(routeGetUserPreferences, map[string]string{"userId": "99"}, 42))
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_SetNotificationPreferences_Unauthorized(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	req := unauthedNotifReq(routeSetUserPreferences, map[string]string{"userId": "42"})
	req.Body = `{"emailEnabled":true,"pushEnabled":true}`

	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestNotificationHandler_SetNotificationPreferences_BadRequest(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"not json", `not json`},
		{"empty body", ``},
		{"missing both fields", `{}`},
		{"missing pushEnabled", `{"emailEnabled":true}`},
		{"missing emailEnabled", `{"pushEnabled":true}`},
		{"explicit null", `{"emailEnabled":null,"pushEnabled":true}`},
		{"wrong type", `{"emailEnabled":"yes","pushEnabled":true}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			aws.AwsOnce.Do(func() {})
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db.SetPoolForTest(mockDB)

			resp, err := NotificationHandler(context.Background(),
				notifReqWithBody(routeSetUserPreferences, map[string]string{"userId": "42"}, 42, tt.body))
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestNotificationHandler_SetNotificationPreferences_InvalidUserID(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeSetUserPreferences, map[string]string{"userId": "abc"}, 42,
			`{"emailEnabled":true,"pushEnabled":true}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

// API Gateway doesn't always populate PathParameters; the handler falls back
// to the raw path segment, and the user routes must too.
func TestNotificationHandler_SetNotificationPreferences_NoPathParameters(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), true, true).
		WillReturnRows(sqlmock.NewRows([]string{"email_enabled", "push_enabled", "notifications_last_seen"}).
			AddRow(true, true, nil))

	req := notifReqWithBody(routeSetUserPreferences, map[string]string{"userId": "42"}, 42,
		`{"emailEnabled":true,"pushEnabled":true}`)
	req.PathParameters = nil
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_SetNotificationPreferences_UnknownUser(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), true, true).
		WillReturnError(&pq.Error{Code: "23503"})

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeSetUserPreferences, map[string]string{"userId": "42"}, 42,
			`{"emailEnabled":true,"pushEnabled":true}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_PatchNotificationsLastSeen(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), lastSeen).
		WillReturnRows(sqlmock.NewRows([]string{"notifications_last_seen"}).AddRow(lastSeen))

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateUserLastSeen, map[string]string{"userId": "42"}, 42,
			`{"notificationsLastSeen":"2026-09-03T14:22:00.000000Z"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"userId":42,"notificationsLastSeen":"2026-09-03T14:22:00Z"}`, resp.Body)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_PatchNotificationsLastSeen_Base64Body(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), lastSeen).
		WillReturnRows(sqlmock.NewRows([]string{"notifications_last_seen"}).AddRow(lastSeen))

	req := notifReqWithBody(routeUpdateUserLastSeen, map[string]string{"userId": "42"}, 42,
		base64.StdEncoding.EncodeToString([]byte(`{"notificationsLastSeen":"2026-09-03T14:22:00Z"}`)))
	req.IsBase64Encoded = true

	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

// The core auth requirement: one user may not write another's timestamp.
// No DB call should be attempted, which mock.ExpectationsWereMet asserts by
// way of no expectations having been registered.
func TestNotificationHandler_PatchNotificationsLastSeen_OtherUserForbidden(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateUserLastSeen, map[string]string{"userId": "99"}, 42,
			`{"notificationsLastSeen":"2026-09-03T14:22:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_PatchNotificationsLastSeen_Unauthorized(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	req := unauthedNotifReq(routeUpdateUserLastSeen, map[string]string{"userId": "42"})
	req.Body = `{"notificationsLastSeen":"2026-09-03T14:22:00Z"}`

	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestNotificationHandler_PatchNotificationsLastSeen_BadRequest(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"not json", `not json`},
		{"empty body", ``},
		{"missing field", `{}`},
		{"explicit null", `{"notificationsLastSeen":null}`},
		{"unparseable timestamp", `{"notificationsLastSeen":"yesterday"}`},
		{"wrong type", `{"notificationsLastSeen":1756909320}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			aws.AwsOnce.Do(func() {})
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db.SetPoolForTest(mockDB)

			resp, err := NotificationHandler(context.Background(),
				notifReqWithBody(routeUpdateUserLastSeen, map[string]string{"userId": "42"}, 42, tt.body))
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestNotificationHandler_PatchNotificationsLastSeen_InvalidUserID(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateUserLastSeen, map[string]string{"userId": "abc"}, 42,
			`{"notificationsLastSeen":"2026-09-03T14:22:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

// API Gateway doesn't always populate PathParameters; the handler falls back
// to the raw path segment, and the user routes must too.
func TestNotificationHandler_PatchNotificationsLastSeen_NoPathParameters(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), lastSeen).
		WillReturnRows(sqlmock.NewRows([]string{"notifications_last_seen"}).AddRow(lastSeen))

	req := notifReqWithBody(routeUpdateUserLastSeen, map[string]string{"userId": "42"}, 42,
		`{"notificationsLastSeen":"2026-09-03T14:22:00Z"}`)
	req.PathParameters = nil
	resp, err := NotificationHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationHandler_PatchNotificationsLastSeen_UnknownUser(t *testing.T) {
	aws.AwsOnce.Do(func() {})
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	lastSeen := time.Date(2026, 9, 3, 14, 22, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO notifications.preferences")).
		WithArgs(int64(42), lastSeen).
		WillReturnError(&pq.Error{Code: "23503"})

	resp, err := NotificationHandler(context.Background(),
		notifReqWithBody(routeUpdateUserLastSeen, map[string]string{"userId": "42"}, 42,
			`{"notificationsLastSeen":"2026-09-03T14:22:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestPathSegmentForParam covers the fallback pathParamInt64 uses when
// PathParameters is empty. It must find the segment whether or not the raw
// path carries the API mapping's /notification base path.
func TestPathSegmentForParam(t *testing.T) {
	cases := []struct {
		name     string
		routeKey string
		rawPath  string
		key      string
		want     string
	}{
		{"with base path", routeSubscribe, "/notification/topic/7/subscription", "topicId", "7"},
		{"without base path", routeSubscribe, "/topic/7/subscription", "topicId", "7"},
		{"trailing slash", routeUpdateSubscription, "/notification/subscription/9/", "subscriptionId", "9"},
		{"user route", routeGetUserPreferences, "/notification/user/42", "userId", "42"},
		{"unknown key", routeGetUserPreferences, "/notification/user/42", "topicId", ""},
		{"raw path shorter than route", routeSubscribe, "/7", "topicId", ""},
		{"malformed route key", "GET", "/notification/user/42", "userId", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, pathSegmentForParam(c.routeKey, c.rawPath, c.key))
		})
	}
}
