package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/aws/aws-lambda-go/events"
)

const (
	defaultNotificationsLimit = 50
	maxNotificationsLimit     = 200
)

// NotificationHandler serves the user subscription and notification
// retrieval API described in terraform/notification-service.yml.
//
// NOTE: unlike WebhookHandler (shared-secret, internal-only), these routes
// are user-facing. The caller's Pennsieve user id is assumed to arrive via a
// JWT authorizer attached to the API Gateway route, surfaced here as the
// "user_id" claim on req.RequestContext.Authorizer.JWT.Claims.
func NotificationHandler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	aws.AwsOnce.Do(func() {
		aws.InitAWS(ctx)
	})

	if err := db.EnsureDB(ctx); err != nil {
		log.Printf("ERROR db init: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "database unavailable"), nil
	}

	userID, err := authenticatedUserID(req)
	if err != nil {
		return notifErrorResponse(http.StatusUnauthorized, "missing or invalid bearer token"), nil
	}

	method := req.RequestContext.HTTP.Method
	path := req.RawPath

	switch {
	case method == http.MethodGet && path == "/notification/topics":
		return handleGetTopics(ctx)
	case method == http.MethodGet && path == "/notification/subscriptions":
		return handleGetSubscriptions(ctx, userID)
	case method == http.MethodPost && strings.HasPrefix(path, "/notification/subscriptions/"):
		return handleSubscribe(ctx, userID, req)
	case method == http.MethodDelete && strings.HasPrefix(path, "/notification/subscriptions/"):
		return handleUnsubscribe(ctx, userID, req)
	case method == http.MethodGet && strings.HasPrefix(path, "/notification/") && strings.HasSuffix(path, "/notifications"):
		return handleGetTopicNotifications(ctx, req)
	default:
		return notifErrorResponse(http.StatusNotFound, "not found"), nil
	}
}

func handleGetTopics(ctx context.Context) (events.APIGatewayV2HTTPResponse, error) {
	topics, err := db.GetTopics(ctx)
	if err != nil {
		log.Printf("ERROR get topics: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to fetch topics"), nil
	}
	return notifJSONResponse(http.StatusOK, nonNilTopics(topics)), nil
}

func handleGetSubscriptions(ctx context.Context, userID int64) (events.APIGatewayV2HTTPResponse, error) {
	subs, err := db.GetUserSubscriptions(ctx, userID)
	if err != nil {
		log.Printf("ERROR get user subscriptions: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to fetch subscriptions"), nil
	}
	return notifJSONResponse(http.StatusOK, nonNilSubscriptions(subs)), nil
}

func handleSubscribe(ctx context.Context, userID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	topicID, err := pathParamInt64(req, "topicId", 2)
	if err != nil {
		return notifErrorResponse(http.StatusBadRequest, "invalid topic id"), nil
	}

	var body models.SubscribeRequest
	if raw, err := decodedBody(req); err != nil {
		return notifErrorResponse(http.StatusBadRequest, "invalid base64 body"), nil
	} else if raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return notifErrorResponse(http.StatusBadRequest, "payload must be valid JSON"), nil
		}
	}

	sub, err := db.CreateSubscription(ctx, userID, topicID, body.Context)
	if err != nil {
		if errors.Is(err, db.ErrTopicNotFound) {
			return notifErrorResponse(http.StatusNotFound, "topic not found"), nil
		}
		log.Printf("ERROR create subscription: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to create subscription"), nil
	}
	return notifJSONResponse(http.StatusCreated, sub), nil
}

func handleUnsubscribe(ctx context.Context, userID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	subscriptionID, err := pathParamInt64(req, "subscriptionId", 2)
	if err != nil {
		return notifErrorResponse(http.StatusBadRequest, "invalid subscription id"), nil
	}

	deleted, err := db.DeleteSubscription(ctx, subscriptionID, userID)
	if err != nil {
		log.Printf("ERROR delete subscription: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to delete subscription"), nil
	}
	if !deleted {
		return notifErrorResponse(http.StatusNotFound, "subscription not found"), nil
	}
	return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusNoContent}, nil
}

func handleGetTopicNotifications(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	topicID, err := pathParamInt64(req, "topicId", 1)
	if err != nil {
		return notifErrorResponse(http.StatusBadRequest, "invalid topic id"), nil
	}

	exists, err := db.TopicExists(ctx, topicID)
	if err != nil {
		log.Printf("ERROR topic exists: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to fetch notifications"), nil
	}
	if !exists {
		return notifErrorResponse(http.StatusNotFound, "topic not found"), nil
	}

	limit, offset := parsePagination(req.QueryStringParameters)

	notifications, err := db.GetTopicNotifications(ctx, topicID, limit, offset)
	if err != nil {
		log.Printf("ERROR get topic notifications: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to fetch notifications"), nil
	}
	return notifJSONResponse(http.StatusOK, nonNilNotifications(notifications)), nil
}

// authenticatedUserID extracts the caller's user id from the "user_id" JWT
// claim attached by the API Gateway authorizer.
func authenticatedUserID(req events.APIGatewayV2HTTPRequest) (int64, error) {
	authorizer := req.RequestContext.Authorizer
	if authorizer == nil || authorizer.JWT == nil {
		return 0, fmt.Errorf("no JWT authorizer context")
	}
	claim, ok := authorizer.JWT.Claims["user_id"]
	if !ok {
		return 0, fmt.Errorf("missing user_id claim")
	}
	return strconv.ParseInt(claim, 10, 64)
}

// pathParamInt64 reads a path parameter by key, falling back to the
// path segment at index (0-based, counted after trimming leading/trailing
// slashes) when API Gateway didn't populate PathParameters.
func pathParamInt64(req events.APIGatewayV2HTTPRequest, key string, index int) (int64, error) {
	value := req.PathParameters[key]
	if value == "" {
		segments := strings.Split(strings.Trim(req.RawPath, "/"), "/")
		if index >= 0 && index < len(segments) {
			value = segments[index]
		}
	}
	return strconv.ParseInt(value, 10, 64)
}

// decodedBody returns req.Body, base64-decoded if necessary.
func decodedBody(req events.APIGatewayV2HTTPRequest) (string, error) {
	if !req.IsBase64Encoded {
		return req.Body, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func parsePagination(params map[string]string) (limit, offset int) {
	limit = defaultNotificationsLimit
	offset = 0
	if v, err := strconv.Atoi(params["limit"]); err == nil && v > 0 && v <= maxNotificationsLimit {
		limit = v
	}
	if v, err := strconv.Atoi(params["offset"]); err == nil && v >= 0 {
		offset = v
	}
	return limit, offset
}

func nonNilTopics(topics []models.Topic) []models.Topic {
	if topics == nil {
		return []models.Topic{}
	}
	return topics
}

func nonNilSubscriptions(subs []models.Subscription) []models.Subscription {
	if subs == nil {
		return []models.Subscription{}
	}
	return subs
}

func nonNilNotifications(notifications []models.Notification) []models.Notification {
	if notifications == nil {
		return []models.Notification{}
	}
	return notifications
}

func notifJSONResponse(statusCode int, body interface{}) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(body)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(b),
	}
}

func notifErrorResponse(statusCode int, message string) events.APIGatewayV2HTTPResponse {
	return notifJSONResponse(statusCode, models.NotificationErrorResponse{
		Code:    statusCode,
		Message: message,
	})
}
