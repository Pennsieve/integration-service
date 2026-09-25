package handler

import (
	"bytes"
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
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
)

const (
	defaultNotificationsLimit = 50
	maxNotificationsLimit     = 200
)

// Route keys of the notifications API gateway (terraform/notification_gateway.tf).
// Its API mapping serves them under the /notification base path, so e.g.
// routeGetTopics is reached at /notification/topics on the API domain.
const (
	routeGetTopics          = "GET /topics"
	routeGetSubscriptions   = "GET /subscriptions"
	routeSubscribe          = "POST /topic/{topicId}/subscription"
	routeUpdateSubscription = "PATCH /subscription/{subscriptionId}"
	routeGetMessages        = "GET /messages"
	routeGetUserPreferences = "GET /user/{userId}"
	routeSetUserPreferences = "POST /user/{userId}"
	routeUpdateUserLastSeen = "PATCH /user/{userId}"
)

// NotificationHandler serves the user subscription and notification
// retrieval API described in terraform/notification-service.yml.
//
// Requests are dispatched on req.RouteKey, the API Gateway route that
// matched, rather than on req.RawPath: the raw path may or may not carry
// the API mapping's /notification base path depending on how the request
// reached the gateway, while the route key never does.
//
// NOTE: unlike WebhookHandler (shared-secret, internal-only), these routes
// are user-facing. The caller's Pennsieve user id arrives via the shared
// Pennsieve Lambda REQUEST authorizer attached to the API Gateway route,
// surfaced here as the "user_claim" context key on
// req.RequestContext.Authorizer.Lambda.
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
		log.Printf("ERROR auth: %v; authorizer=%+v", err, req.RequestContext.Authorizer)
		return notifErrorResponse(http.StatusUnauthorized, "missing or invalid bearer token"), nil
	}

	switch req.RouteKey {
	case routeGetTopics:
		return handleGetTopics(ctx)
	case routeGetSubscriptions:
		return handleGetSubscriptions(ctx, userID)
	case routeSubscribe:
		return handleSubscribe(ctx, userID, req)
	case routeUpdateSubscription:
		return handleUpdateSubscription(ctx, userID, req)
	case routeGetMessages:
		return handleGetMessages(ctx, userID, req)
	case routeGetUserPreferences:
		return handleGetNotificationPreferences(ctx, userID, req)
	case routeSetUserPreferences:
		return handleSetNotificationPreferences(ctx, userID, req)
	case routeUpdateUserLastSeen:
		return handleUpdateNotificationsLastSeen(ctx, userID, req)
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

// handleSubscribe serves POST /notification/topic/{topicId}/subscription. The
// request body is the subscription's context: a free-form JSON object that
// must satisfy the JSON Schema stored as the topic's context, and whose
// referenced dataset (if any) must exist. See validateSubscriptionContext.
// An empty or JSON null body is treated as {}, so it reaches the topic's
// context schema like any other object: a topic with no context accepts it,
// and one whose context has required properties rejects it. A disabled
// topic accepts no new subscriptions (409), and subscribing again to one of
// the caller's own disabled subscriptions re-enables it.
func handleSubscribe(ctx context.Context, userID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	topicID, err := pathParamInt64(req, "topicId")
	if err != nil {
		log.Printf("ERROR subscription validation: invalid topic id: %v", err)
		return notifErrorResponse(http.StatusBadRequest, "invalid topic id"), nil
	}

	raw, err := decodedBody(req)
	if err != nil {
		log.Printf("ERROR subscription validation for topic %d: invalid base64 body: %v", topicID, err)
		return notifErrorResponse(http.StatusBadRequest, "invalid base64 body"), nil
	}
	body := bytes.TrimSpace([]byte(raw))
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		body = []byte("{}")
	}

	topic, err := db.GetTopic(ctx, topicID)
	if err != nil {
		if errors.Is(err, db.ErrTopicNotFound) {
			log.Printf("ERROR subscription validation: topic %d not found", topicID)
			return notifErrorResponse(http.StatusNotFound, "topic not found"), nil
		}
		log.Printf("ERROR get topic: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to create subscription"), nil
	}

	if !topic.Enabled {
		log.Printf("ERROR subscription validation: topic %d (%s) is disabled", topic.TopicID, topic.Name)
		return notifErrorResponse(http.StatusConflict, "topic is disabled"), nil
	}

	if errResp := validateSubscriptionContext(ctx, topic, body); errResp != nil {
		return *errResp, nil
	}

	sub, created, err := db.CreateSubscription(ctx, userID, topicID, body)
	if err != nil {
		if errors.Is(err, db.ErrTopicNotFound) {
			return notifErrorResponse(http.StatusNotFound, "topic not found"), nil
		}
		log.Printf("ERROR create subscription: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to create subscription"), nil
	}
	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated
	}
	return notifJSONResponse(statusCode, sub), nil
}

// handleUpdateSubscription serves PATCH /notification/subscription/{subscriptionId},
// turning one of the caller's own subscriptions on or off. This replaces
// unsubscribing by deleting the subscription: a disabled subscription gets
// no new notifications, but it and the notifications already posted to it
// are kept, so the user's history survives and they can re-enable it later.
func handleUpdateSubscription(ctx context.Context, userID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	subscriptionID, err := pathParamInt64(req, "subscriptionId")
	if err != nil {
		return notifErrorResponse(http.StatusBadRequest, "invalid subscription id"), nil
	}

	body, errResp := decodeJSONBody[models.UpdateSubscriptionRequest](req)
	if errResp != nil {
		return *errResp, nil
	}
	if body.Enabled == nil {
		return notifErrorResponse(http.StatusBadRequest, "enabled is required and must be a boolean"), nil
	}

	sub, err := db.SetSubscriptionEnabled(ctx, subscriptionID, userID, *body.Enabled)
	if err != nil {
		if errors.Is(err, db.ErrSubscriptionNotFound) {
			return notifErrorResponse(http.StatusNotFound, "subscription not found"), nil
		}
		log.Printf("ERROR set subscription enabled: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to update subscription"), nil
	}
	return notifJSONResponse(http.StatusOK, sub), nil
}

// handleGetMessages serves GET /notification/messages: every notification
// posted to any of the caller's subscriptions, newest first and paginated,
// including those posted to subscriptions the caller has since disabled.
func handleGetMessages(ctx context.Context, userID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	limit, offset := parsePagination(req.QueryStringParameters)

	notifications, err := db.GetUserNotifications(ctx, userID, limit, offset)
	if err != nil {
		log.Printf("ERROR get user notifications: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to fetch messages"), nil
	}
	return notifJSONResponse(http.StatusOK, nonNilNotifications(notifications)), nil
}

// handleGetNotificationPreferences serves GET /notification/user/{userId},
// returning that user's email/push notification preferences plus when they
// last viewed their notifications. The notificationsLastSeen field mirrors
// the one on the Pennsieve API's GET /user (see
// docs/notifications-last-seen.md); email/push are read directly from
// notifications.preferences.
func handleGetNotificationPreferences(ctx context.Context, callerID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	targetID, errResp := authorizedTargetUserID(callerID, req)
	if errResp != nil {
		return *errResp, nil
	}

	prefs, err := db.GetNotificationPreferences(ctx, targetID)
	if err != nil {
		log.Printf("ERROR get notification preferences: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to fetch notification preferences"), nil
	}
	return notifJSONResponse(http.StatusOK, prefs), nil
}

// handleSetNotificationPreferences serves POST /notification/user/{userId},
// fully replacing the user's stored email/push notification preferences.
// Both fields are required since this is a full replace, not a partial
// update. It never touches notificationsLastSeen — that field is updated
// only by PATCH /notification/user/{userId}
// (handleUpdateNotificationsLastSeen), since it changes far more often (on
// every notifications-UI open) than a user's channel preferences do.
func handleSetNotificationPreferences(ctx context.Context, callerID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	targetID, errResp := authorizedTargetUserID(callerID, req)
	if errResp != nil {
		return *errResp, nil
	}

	body, errResp := decodeJSONBody[models.SetNotificationPreferencesRequest](req)
	if errResp != nil {
		return *errResp, nil
	}
	if body.EmailEnabled == nil || body.PushEnabled == nil {
		return notifErrorResponse(http.StatusBadRequest, "emailEnabled and pushEnabled are both required"), nil
	}

	prefs, err := db.SetNotificationPreferences(ctx, targetID, *body.EmailEnabled, *body.PushEnabled)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return notifErrorResponse(http.StatusNotFound, "user not found"), nil
		}
		log.Printf("ERROR set notification preferences: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to update notification preferences"), nil
	}
	return notifJSONResponse(http.StatusOK, prefs), nil
}

// handleUpdateNotificationsLastSeen serves PATCH /notification/user/{userId},
// recording that the user viewed their notifications at the supplied
// timestamp. It is a partial update of just this one field, kept separate
// from the full-replace POST preferences route above.
func handleUpdateNotificationsLastSeen(ctx context.Context, callerID int64, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	targetID, errResp := authorizedTargetUserID(callerID, req)
	if errResp != nil {
		return *errResp, nil
	}

	body, errResp := decodeJSONBody[models.UpdateNotificationsLastSeenRequest](req)
	if errResp != nil {
		return *errResp, nil
	}
	if body.NotificationsLastSeen == nil {
		return notifErrorResponse(http.StatusBadRequest, "notificationsLastSeen is required and must be an ISO 8601 timestamp"), nil
	}

	stored, err := db.SetNotificationsLastSeen(ctx, targetID, *body.NotificationsLastSeen)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return notifErrorResponse(http.StatusNotFound, "user not found"), nil
		}
		log.Printf("ERROR update notifications last seen: %v", err)
		return notifErrorResponse(http.StatusInternalServerError, "failed to update notificationsLastSeen"), nil
	}
	return notifJSONResponse(http.StatusOK, models.UserNotificationsLastSeen{
		LastSeen: models.LastSeen{
			UserID:                targetID,
			NotificationsLastSeen: &stored,
		},
	}), nil
}

// authorizedTargetUserID reads the {userId} path parameter and confirms it
// names the caller. A user may only read or write their own notification
// preferences, so any other id is 403 rather than 404: the caller is
// authenticated, just not permitted. Returning 403 for ids that don't exist
// too keeps the route from confirming which user ids are real.
func authorizedTargetUserID(callerID int64, req events.APIGatewayV2HTTPRequest) (int64, *events.APIGatewayV2HTTPResponse) {
	targetID, err := pathParamInt64(req, "userId")
	if err != nil {
		resp := notifErrorResponse(http.StatusBadRequest, "invalid user id")
		return 0, &resp
	}
	if targetID != callerID {
		resp := notifErrorResponse(http.StatusForbidden, "cannot access another user's notification preferences")
		return 0, &resp
	}
	return targetID, nil
}

// authenticatedUserID extracts the caller's user id from the "user_claim"
// context key attached by the shared Pennsieve Lambda authorizer.
func authenticatedUserID(req events.APIGatewayV2HTTPRequest) (int64, error) {
	auth := req.RequestContext.Authorizer
	if auth == nil || auth.Lambda == nil {
		return 0, fmt.Errorf("no lambda authorizer context")
	}
	claims := authorizer.ParseClaims(auth.Lambda)
	if claims == nil || claims.UserClaim == nil {
		return 0, fmt.Errorf("missing user_claim")
	}
	return claims.UserClaim.Id, nil
}

// pathParamInt64 reads a path parameter by key. When API Gateway didn't
// populate PathParameters, it falls back to the matching segment of the raw
// path, located by aligning the matched route's path template (from
// req.RouteKey) with the end of the raw path, so the fallback works whether
// or not the raw path carries the API mapping's base path.
func pathParamInt64(req events.APIGatewayV2HTTPRequest, key string) (int64, error) {
	value := req.PathParameters[key]
	if value == "" {
		value = pathSegmentForParam(req.RouteKey, req.RawPath, key)
	}
	return strconv.ParseInt(value, 10, 64)
}

// pathSegmentForParam returns the segment of rawPath that the {key}
// placeholder in routeKey's path template matched, or "" if there is none.
func pathSegmentForParam(routeKey, rawPath, key string) string {
	_, template, ok := strings.Cut(routeKey, " ")
	if !ok {
		return ""
	}
	templateSegments := strings.Split(strings.Trim(template, "/"), "/")
	pathSegments := strings.Split(strings.Trim(rawPath, "/"), "/")
	offset := len(pathSegments) - len(templateSegments)
	if offset < 0 {
		return ""
	}
	for i, segment := range templateSegments {
		if segment == "{"+key+"}" {
			return pathSegments[offset+i]
		}
	}
	return ""
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

// decodeJSONBody base64-decodes req.Body and unmarshals it into a T,
// sharing the "invalid base64 body" / "payload must be valid JSON" error
// responses across every route with a required JSON body, so the wording
// can't drift between them the way it would with each handler writing its
// own copy of this sequence.
func decodeJSONBody[T any](req events.APIGatewayV2HTTPRequest) (T, *events.APIGatewayV2HTTPResponse) {
	var body T
	raw, err := decodedBody(req)
	if err != nil {
		resp := notifErrorResponse(http.StatusBadRequest, "invalid base64 body")
		return body, &resp
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		resp := notifErrorResponse(http.StatusBadRequest, "payload must be valid JSON")
		return body, &resp
	}
	return body, nil
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
