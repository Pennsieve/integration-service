package handler

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/aws/aws-lambda-go/events"

	cryptorand "crypto/rand"
	"encoding/hex"
)

var allowedMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

var env = os.Getenv("ENV")

const (
	// sharedSecretHeaderName is the HTTP header senders must set with a
	// pre-shared secret. Requests missing it or presenting the wrong value
	// are discarded before any DB work happens.
	sharedSecretHeaderName = "X-Pennsieve-Webhook-Secret"

	// maxPayloadBytes bounds the raw (still-possibly-base64-encoded) request
	// body. Checked before base64 decoding so an oversized body is rejected
	// without spending CPU/memory decoding it first.
	maxPayloadBytes = 1 << 20 // 1 MiB

	// maxRequestsPerSender/senderRateLimitWindow bound how often a single
	// source IP may successfully reach the DB within a sliding window.
	maxRequestsPerSender  = 60
	senderRateLimitWindow = time.Minute
)

var (
	webhookSecretOnce sync.Once
	webhookSecret     string
	webhookSecretErr  error
)

// setSharedSecretForTest overrides the cached webhook shared secret,
// bypassing SSM. Mirrors db.SetPoolForTest.
func setSharedSecretForTest(secret string) {
	webhookSecretOnce = sync.Once{}
	webhookSecretOnce.Do(func() {
		webhookSecret = secret
		webhookSecretErr = nil
	})
}

// ensureWebhookSecret fetches the shared secret once per Lambda execution
// environment and caches it, following the same pattern as db.EnsureDB.
func ensureWebhookSecret(ctx context.Context) (string, error) {
	webhookSecretOnce.Do(func() {
		webhookSecret, webhookSecretErr = aws.GetSSMParam(ctx, fmt.Sprintf("/%s/integration-service/webhook-shared-secret", env), true)
	})
	return webhookSecret, webhookSecretErr
}

// hasValidSharedSecret reports whether the request carries the expected
// shared secret. Header lookup is case-insensitive since API Gateway/Lambda
// event payloads don't guarantee a particular header key casing.
func hasValidSharedSecret(ctx context.Context, headers map[string]string) bool {
	expected, err := ensureWebhookSecret(ctx)
	if err != nil || expected == "" {
		log.Printf("ERROR webhook shared secret unavailable: %v", err)
		return false
	}

	var provided string
	for k, v := range headers {
		if strings.EqualFold(k, sharedSecretHeaderName) {
			provided = v
			break
		}
	}
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func WebhookHandler(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	aws.AwsOnce.Do(func() {
		aws.InitAWS(ctx)
	})

	method := req.RequestContext.HTTP.Method
	if !allowedMethods[method] {
		return errorResponse(http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", method)), nil
	}

	if !hasValidSharedSecret(ctx, req.Headers) {
		return errorResponse(http.StatusUnauthorized, "invalid or missing shared secret"), nil
	}

	if len(req.Body) > maxPayloadBytes {
		return errorResponse(http.StatusRequestEntityTooLarge, "payload too large"), nil
	}

	if err := db.EnsureDB(ctx); err != nil {
		log.Printf("ERROR db init: %v", err)
		return errorResponse(http.StatusInternalServerError, "database unavailable"), nil
	}

	senderIP := req.RequestContext.HTTP.SourceIP
	count, err := db.RecordSenderRequest(ctx, senderIP, senderRateLimitWindow)
	if err != nil {
		log.Printf("ERROR sender rate limit check: %v", err)
		return errorResponse(http.StatusInternalServerError, "failed to process request"), nil
	}
	if count > maxRequestsPerSender {
		return errorResponse(http.StatusTooManyRequests, "rate limit exceeded"), nil
	}

	requestID, err := newUUID()
	if err != nil {
		log.Printf("ERROR uuid: %v", err)
		return errorResponse(http.StatusInternalServerError, "failed to generate request id"), nil
	}

	body := req.Body
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return errorResponse(http.StatusBadRequest, "invalid base64 body"), nil
		}
		body = string(decoded)
	}
	if body == "" {
		body = "{}"
	}
	payload := []byte(body)
	if !json.Valid(payload) {
		return errorResponse(http.StatusBadRequest, "payload must be valid JSON"), nil
	}

	rec, err := db.InsertWebhookMessage(ctx, requestID, payload)
	if err != nil {
		log.Printf("ERROR insert webhook message: %v", err)
		return errorResponse(http.StatusInternalServerError, "failed to store webhook message"), nil
	}

	resp := models.WebhookResponse{
		RequestID:  rec.RequestID,
		ReceivedAt: rec.ReceivedAt,
		Code:       http.StatusAccepted,
		Message:    "accepted",
	}
	return jsonResponse(http.StatusAccepted, resp), nil
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:]),
	), nil
}

func jsonResponse(statusCode int, body interface{}) events.LambdaFunctionURLResponse {
	b, _ := json.Marshal(body)
	return events.LambdaFunctionURLResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(b),
	}
}

func errorResponse(statusCode int, message string) events.LambdaFunctionURLResponse {
	resp := models.WebhookResponse{
		Code:    statusCode,
		Message: message,
	}
	return jsonResponse(statusCode, resp)
}
