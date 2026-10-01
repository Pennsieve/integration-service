// Package notification_matcher finds the subscriptions an event notifies.
//
// A subscription matches when its stored context contains the context
// extracted from the event (subscription.context @> extracted). Because
// containment only holds when every extracted key is also in the
// subscription, what the extractor returns decides what can match:
//
//   - It returns exactly the topic's required context fields, never optional
//     ones. Every subscription carries the required fields, since
//     subscriptions are validated against the topic's context schema, but an
//     optional one would silently exclude every subscription that left it
//     out.
//   - A topic with no context (or no required fields) extracts {}, which
//     every subscription to that topic contains.
//   - Values are converted to the types subscriptions store them as. JSONB
//     containment compares types strictly, so {"organizationId":45} does not
//     contain {"organizationId":"45"}, and events send ids as strings.
package notification_matcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Pennsieve/integration-service/internal/models"
)

// ErrBadField reports that an event is missing a field the topic's context
// requires, or carries one that can't be converted to the type subscriptions
// store it as. Such an event can't match any subscription to the topic, and
// retrying it won't change that, so callers should log and skip it.
var ErrBadField = errors.New("bad event field")

// fieldExtractors maps each context field an event can supply to how it is
// read from the event. Each returns the value in the type subscription
// context stores it as: organizationId and datasetId are validated as
// positive integers when a subscription is created.
var fieldExtractors = map[string]func(models.EventMessage) (any, error){
	"organizationId": func(e models.EventMessage) (any, error) {
		if e.OrgID == "" {
			return nil, errors.New("missing")
		}
		return positiveInt64(e.OrgID)
	},
	"datasetId": func(e models.EventMessage) (any, error) {
		if e.DatasetID == nil {
			return nil, errors.New("missing")
		}
		if *e.DatasetID <= 0 {
			return nil, fmt.Errorf("got %d, want a positive integer", *e.DatasetID)
		}
		return int64(*e.DatasetID), nil
	},
}

// ExtractContext returns the subscription context an event must be
// contained in to notify a subscription to topic: the topic's required
// context fields, read from the event. It returns an error wrapping
// ErrBadField when the event can't supply one of them.
func ExtractContext(topic models.Topic, event models.EventMessage) (map[string]any, error) {
	required, err := requiredFields(topic.Context)
	if err != nil {
		return nil, fmt.Errorf("topic %d (%s) context: %w", topic.TopicID, topic.Name, err)
	}

	extracted := make(map[string]any, len(required))
	for _, field := range required {
		extract, ok := fieldExtractors[field]
		if !ok {
			return nil, fmt.Errorf("%w: topic %d (%s) requires %q, which events don't carry",
				ErrBadField, topic.TopicID, topic.Name, field)
		}
		v, err := extract(event)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrBadField, field, err)
		}
		extracted[field] = v
	}
	return extracted, nil
}

// requiredFields returns the "required" list of a topic's context JSON
// Schema, or nothing for a topic with no context.
//
// It trusts that a topic's context is a JSON Schema. Nothing enforces that
// yet: topic contexts are hand-maintained. Valid JSON of another shape, such
// as an example payload, has no "required" list and so extracts {}, which
// matches every subscription to the topic. Whatever path comes to create or
// edit topics must validate that a declared context is a JSON Schema whose
// "required" list parses and is non-empty.
func requiredFields(topicContext json.RawMessage) ([]string, error) {
	trimmed := bytes.TrimSpace(topicContext)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(trimmed, &schema); err != nil {
		return nil, err
	}
	return schema.Required, nil
}

// positiveInt64 parses an id sent as a string, such as "45".
func positiveInt64(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot convert %q to an integer", s)
	}
	if n <= 0 {
		return 0, fmt.Errorf("got %d, want a positive integer", n)
	}
	return n, nil
}
