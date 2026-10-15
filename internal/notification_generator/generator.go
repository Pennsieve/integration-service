// Package notification_generator turns incoming events into notification
// rows for the subscriptions they match.
//
// Every event is matched to the topic named after its event type, then
// against that topic's subscriptions (notification_matcher), and recorded
// once per matched subscription. An event type with no topic generates
// nothing. The notification's content comes from the event type's renderer
// in renderers, or from defaultRenderer when it has none. Rows are written
// regardless of the topic's and subscriptions' enabled flags, which gate sending rather than recording
// (docs/notification-status-flags-scope.md).
//
// After recording, each event's deliveries are planned (planDeliveries):
// which matched subscriptions are to be sent to, and on which channels. A
// subscription's own notification_channels decide that when it has them,
// and its owner's preferences otherwise. Email deliveries are then sent
// (sendEmails) through the email-service; push and SMS have no sender yet,
// so those deliveries are only counted in the logs.
package notification_generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/email"
	"github.com/Pennsieve/integration-service/internal/event_parser"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/Pennsieve/integration-service/internal/notification_matcher"
)

// errBadDetail reports an event whose eventDetail can't be rendered. Like
// notification_matcher.ErrBadField, retrying won't fix it.
var errBadDetail = errors.New("bad event detail")

// rendered is the content of the notification an event posts.
type rendered struct {
	Title    string
	Message  string
	Metadata json.RawMessage
}

// renderer describes how to render one event type's notifications: the
// eventCategory it must arrive with ("" accepts any), and the render
// function.
type renderer struct {
	category string
	render   func(models.EventMessage) (rendered, error)
}

// renderers holds the type-specific renderers, keyed by eventType (which is
// also the name of the topic the event notifies). Event types missing here
// use defaultRenderer.
//
// The workspace-wide publishing broadcasts each get their own renderer, one
// per lifecycle outcome; none stands in for another. They share the
// PUBLISHING category with the dataset-scoped changelog events
// (COMPLETE_PUBLICATION, ...), which are told apart by event type, not
// category.
var renderers = map[string]renderer{
	models.EventTypeDatasetPublished: publicationRenderer("Dataset published",
		func(d models.DatasetPublicationDetail) string {
			return fmt.Sprintf("%q (version %d) was published to Pennsieve Discover with DOI %s.",
				d.DatasetName, d.PublishedVersion, d.DOI)
		}),
	models.EventTypeDatasetEmbargoed: publicationRenderer("Dataset embargoed",
		func(d models.DatasetPublicationDetail) string {
			return fmt.Sprintf("%q (version %d) was published to Pennsieve Discover under embargo with DOI %s.",
				d.DatasetName, d.PublishedVersion, d.DOI)
		}),
	models.EventTypeDatasetEmbargoReleased: publicationRenderer("Dataset embargo released",
		func(d models.DatasetPublicationDetail) string {
			return fmt.Sprintf("The embargo on %q (version %d) has ended, and it is now public on Pennsieve Discover with DOI %s.",
				d.DatasetName, d.PublishedVersion, d.DOI)
		}),
	models.EventTypeDatasetUnpublished: publicationRenderer("Dataset unpublished",
		func(d models.DatasetPublicationDetail) string {
			return fmt.Sprintf("%q (DOI %s) was removed from Pennsieve Discover.", d.DatasetName, d.DOI)
		}),
	models.EventTypeDatasetRevised: publicationRenderer("Dataset revised",
		func(d models.DatasetPublicationDetail) string {
			return fmt.Sprintf("The published metadata of %q (version %d, DOI %s) was revised on Pennsieve Discover.",
				d.DatasetName, d.PublishedVersion, d.DOI)
		}),
}

// defaultRenderer renders any event type without a type-specific renderer,
// in whatever category it arrives with.
var defaultRenderer = renderer{render: renderDefault}

// rendererFor returns eventType's renderer, falling back to defaultRenderer.
func rendererFor(eventType string) renderer {
	if r, ok := renderers[eventType]; ok {
		return r
	}
	return defaultRenderer
}

// Generate records a notification for every subscription each event in
// mapped (as grouped by event_parser.MapEvents) matches, and emails those
// whose delivery channels include email through emails. A nil emails means
// email isn't configured for this environment, and no email is sent.
//
// It never fails the batch. An event that can't be rendered or matched is
// logged with event_parser.SkippedRecordMarker, which alerting counts, and
// skipped. A database error is logged and that event dropped too: returning
// it would make Lambda retry the whole batch, re-sending every webhook in it.
// For the same reason a failed email is logged and the rest still sent.
func Generate(ctx context.Context, mapped map[string][]models.EventMessage, emails email.Sender) {
	topics := map[string]*models.Topic{}
	for _, events := range mapped {
		for _, event := range events {
			n, err := generate(ctx, topics, event, rendererFor(event.Type), emails)
			switch {
			case errors.Is(err, errBadDetail), errors.Is(err, notification_matcher.ErrBadField):
				log.Printf("WARN %s %s event for organization %q: %v",
					event_parser.SkippedRecordMarker, event.Type, event.OrgID, err)
			case err != nil:
				log.Printf("ERROR generate notifications for %s event for organization %q: %v",
					event.Type, event.OrgID, err)
			case n > 0:
				log.Printf("Recorded %d notification(s) for %s event for organization %s",
					n, event.Type, event.OrgID)
			}
		}
	}
}

// generate records the notifications for one event, rendered by r, sends
// their emails through emails, and returns how many notifications were
// written. The caller resolves r (see rendererFor) so
// it isn't looked up again here. topics caches topic lookups for the batch;
// a nil entry records a topic that doesn't exist.
func generate(ctx context.Context, topics map[string]*models.Topic, event models.EventMessage, r renderer, emails email.Sender) (int64, error) {
	if r.category != "" && event.Category != r.category {
		return 0, fmt.Errorf("%w: eventCategory %q, want %q", errBadDetail, event.Category, r.category)
	}
	// Rendered before matching so a malformed detail is reported even while
	// nothing subscribes to the topic.
	content, err := r.render(event)
	if err != nil {
		return 0, err
	}

	topic, cached := topics[event.Type]
	if !cached {
		// Most event types have no topic, so a missing one isn't logged.
		t, err := db.GetTopicByName(ctx, event.Type)
		switch {
		case errors.Is(err, db.ErrTopicNotFound):
		case err != nil:
			return 0, err
		default:
			topic = &t
		}
		topics[event.Type] = topic
	}
	if topic == nil {
		return 0, nil
	}

	subs, err := notification_matcher.Match(ctx, *topic, event)
	if err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}

	ids := make([]int64, len(subs))
	for i, s := range subs {
		ids[i] = s.SubscriptionID
	}
	notificationIDs, err := db.CreateNotifications(ctx, ids, content.Title, content.Message, content.Metadata)
	if err != nil {
		return 0, err
	}
	n := int64(len(notificationIDs))

	deliveries, err := planDeliveries(ctx, *topic, subs)
	if err != nil {
		return n, fmt.Errorf("plan deliveries: %w", err)
	}
	logDeliveries(event, deliveries)
	if err := sendEmails(ctx, emails, *topic, event, content, deliveries, notificationIDs); err != nil {
		return n, fmt.Errorf("send emails: %w", err)
	}
	return n, nil
}

// delivery is one subscription's notification as it is to be sent: the
// channels it goes out on, already resolved from the subscription's own
// override or its owner's preferences.
type delivery struct {
	SubscriptionID int64
	UserID         int64
	Channels       models.NotificationChannels
}

// planDeliveries decides which of subs, all matched to topic by one event,
// are sent to and on which channels.
//
// A subscription is sent to only while it is effectively enabled, i.e.
// topic.Enabled AND its own Enabled (docs/notification-status-flags-scope.md).
// Its channels are its NotificationChannels when set, overriding the user's
// preferences entirely, and otherwise its owner's preferences, read in one
// query for every owner that needs them. A subscription whose resolved
// channels are all off has nothing to send and is left out.
func planDeliveries(ctx context.Context, topic models.Topic, subs []models.Subscription) ([]delivery, error) {
	if !topic.Enabled {
		return nil, nil
	}

	var fallbackUsers []int64
	seen := map[int64]bool{}
	for _, s := range subs {
		if s.Enabled && s.NotificationChannels == nil && !seen[s.UserID] {
			seen[s.UserID] = true
			fallbackUsers = append(fallbackUsers, s.UserID)
		}
	}
	preferences, err := db.GetUserNotificationChannels(ctx, fallbackUsers)
	if err != nil {
		return nil, err
	}

	var deliveries []delivery
	for _, s := range subs {
		if !s.Enabled {
			continue
		}
		channels := preferences[s.UserID]
		if s.NotificationChannels != nil {
			channels = *s.NotificationChannels
		}
		if !channels.Any() {
			continue
		}
		deliveries = append(deliveries, delivery{SubscriptionID: s.SubscriptionID, UserID: s.UserID, Channels: channels})
	}
	return deliveries, nil
}

// logDeliveries logs how many of an event's deliveries go out on each
// channel. Only email has a sender (sendEmails); push and SMS deliveries are
// counted here so their volume is visible before they get one.
func logDeliveries(event models.EventMessage, deliveries []delivery) {
	if len(deliveries) == 0 {
		return
	}
	var email, push, sms int
	for _, d := range deliveries {
		if d.Channels.Email {
			email++
		}
		if d.Channels.Push {
			push++
		}
		if d.Channels.SMS {
			sms++
		}
	}
	log.Printf("Planned %d delivery(ies) for %s event for organization %s: email=%d push=%d sms=%d",
		len(deliveries), event.Type, event.OrgID, email, push, sms)
}

// publicationRenderer renders one of the workspace-wide publishing
// broadcasts, which all carry a models.DatasetPublicationDetail and must
// arrive in the PUBLISHING category. The title is titlePrefix and the
// dataset name; the body is message's sentence, plus the dataset owner when
// the detail names one. The decoded detail is stored as the notification's
// metadata, giving clients the ids, node id and DOI to link to.
func publicationRenderer(titlePrefix string, message func(models.DatasetPublicationDetail) string) renderer {
	return renderer{
		category: models.EventCategoryPublishing,
		render: func(event models.EventMessage) (rendered, error) {
			d, err := decodePublicationDetail(event)
			if err != nil {
				return rendered{}, err
			}
			body := message(d)
			if d.OwnerName != "" {
				body += fmt.Sprintf(" Dataset owner: %s.", d.OwnerName)
			}
			metadata, err := json.Marshal(d)
			if err != nil {
				return rendered{}, fmt.Errorf("encode metadata: %w", err)
			}
			return rendered{Title: titlePrefix + ": " + d.DatasetName, Message: body, Metadata: metadata}, nil
		},
	}
}

// decodePublicationDetail decodes and validates a publishing broadcast's
// eventDetail.
func decodePublicationDetail(event models.EventMessage) (models.DatasetPublicationDetail, error) {
	var d models.DatasetPublicationDetail
	if len(event.Detail) == 0 {
		return d, fmt.Errorf("%w: eventDetail missing", errBadDetail)
	}
	if err := json.Unmarshal(event.Detail, &d); err != nil {
		return d, fmt.Errorf("%w: %v", errBadDetail, err)
	}
	switch {
	case d.DatasetID <= 0:
		return d, fmt.Errorf("%w: datasetId %d, want a positive integer", errBadDetail, d.DatasetID)
	case d.DatasetName == "":
		return d, fmt.Errorf("%w: datasetName missing", errBadDetail)
	case d.PublishedVersion <= 0:
		return d, fmt.Errorf("%w: publishedVersion %d, want a positive integer", errBadDetail, d.PublishedVersion)
	case d.DOI == "":
		return d, fmt.Errorf("%w: doi missing", errBadDetail)
	}
	return d, nil
}

// renderDefault renders an event that has no type-specific renderer. The
// title is the event type, and both the message and the metadata are the
// event body as single-line JSON: the envelope fields plus eventDetail as
// sent, so nothing in the event is lost even though nothing in it is
// interpreted.
func renderDefault(event models.EventMessage) (rendered, error) {
	body, err := json.Marshal(struct {
		OrgID     string          `json:"organizationId,omitempty"`
		DatasetID *int            `json:"datasetId,omitempty"`
		SubjectID *int            `json:"subjectUserId,omitempty"`
		Category  string          `json:"eventCategory,omitempty"`
		Type      string          `json:"eventType"`
		Detail    json.RawMessage `json:"eventDetail,omitempty"`
	}{event.OrgID, event.DatasetID, event.SubjectUserID, event.Category, event.Type, event.Detail})
	if err != nil {
		return rendered{}, fmt.Errorf("%w: %v", errBadDetail, err)
	}
	return rendered{Title: event.Type, Message: string(body), Metadata: body}, nil
}

// sendEmails emails every delivery whose channels include email, through
// emails, as the email-service "user-notification" email (see
// email.NewRequest). Each email's dedupe id comes from the notification row
// written for its subscription (notificationIDs), so enqueuing it again can't
// send it twice.
//
// A user gets one email per event however many of their subscriptions the
// event matched: the first such delivery (subs come ordered by
// subscription_id) is the one sent. A user with no email address on
// pennsieve.users is skipped with a warning, and a failed send is logged and
// the rest still sent; only failing to look the recipients up at all is
// returned.
func sendEmails(ctx context.Context, emails email.Sender, topic models.Topic, event models.EventMessage, content rendered, deliveries []delivery, notificationIDs map[int64]int64) error {
	var toSend []delivery
	seen := map[int64]bool{}
	for _, d := range deliveries {
		if d.Channels.Email && !seen[d.UserID] {
			seen[d.UserID] = true
			toSend = append(toSend, d)
		}
	}
	if len(toSend) == 0 {
		return nil
	}
	if emails == nil {
		log.Printf("WARN email is not configured (%s unset); skipping %d email(s) for %s event for organization %s",
			email.QueueURLEnv, len(toSend), event.Type, event.OrgID)
		return nil
	}

	userIDs := make([]int64, len(toSend))
	for i, d := range toSend {
		userIDs[i] = d.UserID
	}
	recipients, err := db.GetEmailRecipients(ctx, userIDs)
	if err != nil {
		return err
	}

	var sent int
	for _, d := range toSend {
		to, ok := recipients[d.UserID]
		if !ok {
			log.Printf("WARN no email address for user %d; skipping email for subscription %d", d.UserID, d.SubscriptionID)
			continue
		}
		req := email.NewRequest(to, email.Notification{
			NotificationID: notificationIDs[d.SubscriptionID],
			OrganizationID: event.OrgID,
			Topic:          topic,
			Title:          content.Title,
			Message:        content.Message,
		})
		if err := emails.Send(ctx, req); err != nil {
			log.Printf("ERROR send notification email for subscription %d to user %d: %v", d.SubscriptionID, d.UserID, err)
			continue
		}
		sent++
	}
	log.Printf("Sent %d of %d notification email(s) for %s event for organization %s",
		sent, len(toSend), event.Type, event.OrgID)
	return nil
}
