package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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

func WebhookHandler(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	aws.AwsOnce.Do(func() {
		aws.InitAWS(ctx)
	})

	if err := db.EnsureDB(ctx); err != nil {
		log.Printf("ERROR db init: %v", err)
		return errorResponse(http.StatusInternalServerError, "database unavailable"), nil
	}

	method := req.RequestContext.HTTP.Method
	if !allowedMethods[method] {
		return errorResponse(http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", method)), nil
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
	const maxBodyBytes = 1 << 20
	if len(payload) > maxBodyBytes {
		return errorResponse(http.StatusBadRequest, "payload too large"), nil
	}
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
