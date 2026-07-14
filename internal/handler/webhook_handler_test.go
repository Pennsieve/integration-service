package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSharedSecret = "test-shared-secret"

func markAWSReady() {
	aws.AwsOnce.Do(func() {})
	setSharedSecretForTest(testSharedSecret)
}

func lambdaReq(method, body string) events.LambdaFunctionURLRequest {
	return events.LambdaFunctionURLRequest{
		Body: body,
		Headers: map[string]string{
			sharedSecretHeaderName: testSharedSecret,
		},
		RequestContext: events.LambdaFunctionURLRequestContext{
			HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
				Method:   method,
				SourceIP: "203.0.113.10",
			},
		},
	}
}

func lambdaReqBase64(method, body string) events.LambdaFunctionURLRequest {
	req := lambdaReq(method, body)
	req.IsBase64Encoded = true
	return req
}

// expectRateLimitQuery sets up the sqlmock expectation for the
// webhooks.sender_rate_limits upsert that every request past the shared
// secret and size checks must go through.
func expectRateLimitQuery(mock sqlmock.Sqlmock, count int) {
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO webhooks.sender_rate_limits")).
		WillReturnRows(sqlmock.NewRows([]string{"request_count"}).AddRow(count))
}

func TestNewUUID(t *testing.T) {
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	ids := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id, err := newUUID()
		require.NoError(t, err)
		assert.Regexp(t, uuidRe, id, "UUID must match v4 format")
		assert.False(t, ids[id], "UUID must be unique")
		ids[id] = true
	}
}

func TestJsonResponse(t *testing.T) {
	payload := map[string]string{"hello": "world"}
	resp := jsonResponse(http.StatusOK, payload)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Headers["Content-Type"])

	var got map[string]string
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &got))
	assert.Equal(t, "world", got["hello"])
}

func TestErrorResponse(t *testing.T) {
	resp := errorResponse(http.StatusBadRequest, "oops")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var got models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &got))
	assert.Equal(t, http.StatusBadRequest, got.Code)
	assert.Equal(t, "oops", got.Message)
}

func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace} {
		t.Run(method, func(t *testing.T) {
			resp, err := WebhookHandler(context.Background(), lambdaReq(method, ""))
			require.NoError(t, err)
			assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

			var body models.WebhookResponse
			require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
			assert.Contains(t, body.Message, "not allowed")
		})
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_InvalidJSON(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()
	expectRateLimitQuery(mock, 1)

	resp, err := WebhookHandler(context.Background(), lambdaReq(http.MethodPost, "not-json"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Contains(t, body.Message, "valid JSON")
}

func TestWebhookHandler_EmptyBody(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

	now := time.Now()
	expectRateLimitQuery(mock, 1)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO webhooks.messages")).
		WithArgs(sqlmock.AnyArg(), []byte("{}")).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "request_id", "payload", "received_at"}).
				AddRow(1, "test-uuid", []byte("{}"), now),
		)

	resp, err := WebhookHandler(context.Background(), lambdaReq(http.MethodPost, ""))
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Equal(t, "accepted", body.Message)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_Success(t *testing.T) {
	for _, method := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		t.Run(method, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db.SetPoolForTest(mockDB)
			markAWSReady()

			payload := `{"event":"test"}`
			now := time.Now()
			reqID := fmt.Sprintf("uuid-%s", method)

			expectRateLimitQuery(mock, 1)
			mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO webhooks.messages")).
				WithArgs(sqlmock.AnyArg(), []byte(payload)).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "request_id", "payload", "received_at"}).
						AddRow(1, reqID, []byte(payload), now),
				)

			resp, err := WebhookHandler(context.Background(), lambdaReq(method, payload))
			require.NoError(t, err)
			assert.Equal(t, http.StatusAccepted, resp.StatusCode)

			var body models.WebhookResponse
			require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
			assert.Equal(t, http.StatusAccepted, body.Code)
			assert.Equal(t, "accepted", body.Message)
			assert.Equal(t, reqID, body.RequestID)
			assert.WithinDuration(t, now, body.ReceivedAt, time.Second)

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWebhookHandler_Base64EncodedBody(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

	payload := `{"event":"test"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	now := time.Now()

	expectRateLimitQuery(mock, 1)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO webhooks.messages")).
		WithArgs(sqlmock.AnyArg(), []byte(payload)).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "request_id", "payload", "received_at"}).
				AddRow(1, "test-uuid", []byte(payload), now),
		)

	resp, err := WebhookHandler(context.Background(), lambdaReqBase64(http.MethodPost, encoded))
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Equal(t, "accepted", body.Message)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_Base64EncodedBody_Invalid(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()
	expectRateLimitQuery(mock, 1)

	resp, err := WebhookHandler(context.Background(), lambdaReqBase64(http.MethodPost, "not-valid-base64!!"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Contains(t, body.Message, "invalid base64")
}

// TestWebhookHandler_DBInsertFailure verifies a 500 is returned when the DB insert fails.
func TestWebhookHandler_DBInsertFailure(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

	expectRateLimitQuery(mock, 1)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO webhooks.messages")).
		WillReturnError(fmt.Errorf("connection reset"))

	resp, err := WebhookHandler(context.Background(), lambdaReq(http.MethodPost, `{"k":"v"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Contains(t, body.Message, "store webhook message")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_MissingSharedSecret(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

	req := lambdaReq(http.MethodPost, `{"k":"v"}`)
	delete(req.Headers, sharedSecretHeaderName)

	resp, err := WebhookHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Contains(t, body.Message, "shared secret")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_InvalidSharedSecret(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

	req := lambdaReq(http.MethodPost, `{"k":"v"}`)
	req.Headers[sharedSecretHeaderName] = "wrong-secret"

	resp, err := WebhookHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Contains(t, body.Message, "shared secret")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_SharedSecretHeaderIsCaseInsensitive(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()
	expectRateLimitQuery(mock, 1)

	payload := `{"event":"test"}`
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO webhooks.messages")).
		WithArgs(sqlmock.AnyArg(), []byte(payload)).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "request_id", "payload", "received_at"}).
				AddRow(1, "test-uuid", []byte(payload), now),
		)

	req := lambdaReq(http.MethodPost, payload)
	delete(req.Headers, sharedSecretHeaderName)
	req.Headers[strings.ToLower(sharedSecretHeaderName)] = testSharedSecret

	resp, err := WebhookHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_PayloadTooLarge(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

	oversized := strings.Repeat("a", maxPayloadBytes+1)
	resp, err := WebhookHandler(context.Background(), lambdaReq(http.MethodPost, oversized))
	require.NoError(t, err)
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Contains(t, body.Message, "too large")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWebhookHandler_RateLimitExceeded(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()
	expectRateLimitQuery(mock, maxRequestsPerSender+1)

	resp, err := WebhookHandler(context.Background(), lambdaReq(http.MethodPost, `{"k":"v"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	var body models.WebhookResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Contains(t, body.Message, "rate limit")
	require.NoError(t, mock.ExpectationsWereMet())
}
