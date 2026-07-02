package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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

func markAWSReady() {
	aws.AwsOnce.Do(func() {})
}

func lambdaReq(method, body string) events.LambdaFunctionURLRequest {
	return events.LambdaFunctionURLRequest{
		Body: body,
		RequestContext: events.LambdaFunctionURLRequestContext{
			HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
				Method: method,
			},
		},
	}
}

func lambdaReqBase64(method, body string) events.LambdaFunctionURLRequest {
	req := lambdaReq(method, body)
	req.IsBase64Encoded = true
	return req
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
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

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
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)
	markAWSReady()

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
