// Package notification_generator turns incoming events into notification
// rows for the subscriptions they match.
//
// Only event types with a renderer below generate notifications. Each one
// is matched to the topic named after its event type, matched against that
// topic's subscriptions (notification_matcher), and recorded once per
// matched subscription. Rows are written regardless of the topic's and
// subscriptions' enabled flags, which gate sending rather than recording
// (docs/notification-status-flags-scope.md); there is no send step yet.
package notification_generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/Pennsieve/integration-service/internal/db"
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

// renderer describes one event type that generates notifications: the
// eventCategory it must arrive with, and how to render it.
type renderer struct {
	category string
	render   func(models.EventMessage) (rendered, error)
}

// renderers is keyed by eventType, which is also the name of the topic the
// event notifies.
//
// DATASET_PUBLISHED_IN_WORKSPACE is checked against the ORGANIZATION
// category so it can't be confused with the dataset-scoped PUBLISHING
// changelog events (REQUEST_PUBLICATION, COMPLETE_PUBLICATION, ...), which
// notify per-dataset subscribers, not the whole workspace.
var renderers = map[string]renderer{
	models.EventTypeDatasetPublishedInWorkspace: {
		category: models.EventCategoryOrganization,
		render:   renderDatasetPublishedInWorkspace,
	},
}

// Generate records a notification for every subscription each event in
// mapped (as grouped by event_parser.MapEvents) matches.
//
// It never fails the batch. An event that can't be rendered or matched is
// logged with event_parser.SkippedRecordMarker, which alerting counts, and
// skipped. A database error is logged and that event dropped too: returning
// it would make Lambda retry the whole batch, re-sending every webhook in it.
func Generate(ctx context.Context, mapped map[string][]models.EventMessage) {
	topics := map[string]*models.Topic{}
	for _, events := range mapped {
		for _, event := range events {
			if _, ok := renderers[event.Type]; !ok {
				continue
			}
			n, err := generate(ctx, topics, event)
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

// generate records the notifications for one event that has a renderer,
// returning how many were written. topics caches topic lookups for the
// batch; a nil entry records a topic that doesn't exist.
func generate(ctx context.Context, topics map[string]*models.Topic, event models.EventMessage) (int64, error) {
	r := renderers[event.Type]
	if event.Category != r.category {
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
		t, err := db.GetTopicByName(ctx, event.Type)
		switch {
		case errors.Is(err, db.ErrTopicNotFound):
			log.Printf("WARN no topic named %s; not generating notifications for it", event.Type)
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
	return db.CreateNotifications(ctx, ids, content.Title, content.Message, content.Metadata)
}

// renderDatasetPublishedInWorkspace renders a DATASET_PUBLISHED_IN_WORKSPACE
// event. It fires for revisions as well as first publications, so the copy
// names the version when it isn't the first. The decoded detail is stored as
// the notification's metadata, giving clients the ids, node id and DOI to
// link to.
func renderDatasetPublishedInWorkspace(event models.EventMessage) (rendered, error) {
	if len(event.Detail) == 0 {
		return rendered{}, fmt.Errorf("%w: eventDetail missing", errBadDetail)
	}
	var d models.DatasetPublishedInWorkspaceDetail
	if err := json.Unmarshal(event.Detail, &d); err != nil {
		return rendered{}, fmt.Errorf("%w: %v", errBadDetail, err)
	}
	switch {
	case d.DatasetID <= 0:
		return rendered{}, fmt.Errorf("%w: datasetId %d, want a positive integer", errBadDetail, d.DatasetID)
	case d.DatasetName == "":
		return rendered{}, fmt.Errorf("%w: datasetName missing", errBadDetail)
	case d.PublishedVersion <= 0:
		return rendered{}, fmt.Errorf("%w: publishedVersion %d, want a positive integer", errBadDetail, d.PublishedVersion)
	case d.DOI == "":
		return rendered{}, fmt.Errorf("%w: doi missing", errBadDetail)
	}

	message := fmt.Sprintf("%q was published to Pennsieve Discover with DOI %s.", d.DatasetName, d.DOI)
	if d.PublishedVersion > 1 {
		message = fmt.Sprintf("Version %d of %q was published to Pennsieve Discover with DOI %s.",
			d.PublishedVersion, d.DatasetName, d.DOI)
	}
	if d.OwnerName != "" {
		message += fmt.Sprintf(" Dataset owner: %s.", d.OwnerName)
	}

	metadata, err := json.Marshal(d)
	if err != nil {
		return rendered{}, fmt.Errorf("encode metadata: %w", err)
	}
	return rendered{
		Title:    "Dataset published: " + d.DatasetName,
		Message:  message,
		Metadata: metadata,
	}, nil
}
