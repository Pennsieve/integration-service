package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/aws/aws-lambda-go/events"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// topicContextSchemaURL is the location the topic's context schema is
// registered under when compiling it. It is never fetched; it only names the
// in-memory resource, and shows up in validation error messages.
const topicContextSchemaURL = "urn:pennsieve:topic-context"

// errInvalidTopicSchema reports that a topic's stored context is not a
// usable JSON Schema. That is a server-side misconfiguration of the topic,
// not a problem with the caller's request.
var errInvalidTopicSchema = errors.New("invalid topic context schema")

// validateSubscriptionContext checks a create-subscription request body
// against the topic being subscribed to, in order:
//
//  1. the body is a JSON object,
//  2. it satisfies the JSON Schema stored as the topic's context (skipped
//     for a topic with no context), and
//  3. any dataset it references (organizationId + datasetId) exists.
//
// It returns nil when the body is valid, or the error response to send.
// Every failure is logged along with the validation error that caused it.
func validateSubscriptionContext(ctx context.Context, topic models.Topic, body []byte) *events.APIGatewayV2HTTPResponse {
	fail := func(statusCode int, message string, cause error) *events.APIGatewayV2HTTPResponse {
		log.Printf("ERROR subscription validation for topic %d (%s): %s: %v", topic.TopicID, topic.Name, message, cause)
		resp := notifErrorResponse(statusCode, message)
		return &resp
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return fail(http.StatusBadRequest, "payload must be valid JSON", err)
	}
	fields, ok := instance.(map[string]any)
	if !ok {
		return fail(http.StatusBadRequest, "payload must be a JSON object", fmt.Errorf("got %T", instance))
	}

	if err := validateAgainstTopicContext(topic.Context, instance); err != nil {
		if errors.Is(err, errInvalidTopicSchema) {
			return fail(http.StatusInternalServerError, "topic context schema is invalid", err)
		}
		return fail(http.StatusBadRequest, "payload does not match the topic context: "+validationMessage(err), err)
	}

	if resp := validateDatasetReference(ctx, fields, fail); resp != nil {
		return resp
	}
	return nil
}

// validateAgainstTopicContext validates instance (already decoded with
// jsonschema.UnmarshalJSON, so numbers are json.Number) against the topic's
// context JSON Schema. A topic with no context accepts any object.
func validateAgainstTopicContext(topicContext json.RawMessage, instance any) error {
	trimmed := bytes.TrimSpace(topicContext)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(trimmed))
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidTopicSchema, err)
	}
	// Topics declare "$schema": "http://json-schema.org", which names no
	// actual draft, and the compiler would otherwise try to fetch whatever
	// $schema points at as a metaschema. Drop it and compile against the
	// compiler's default draft instead.
	if m, ok := schemaDoc.(map[string]any); ok {
		delete(m, "$schema")
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(topicContextSchemaURL, schemaDoc); err != nil {
		return fmt.Errorf("%w: %v", errInvalidTopicSchema, err)
	}
	schema, err := c.Compile(topicContextSchemaURL)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidTopicSchema, err)
	}
	return schema.Validate(instance)
}

// validationMessage flattens a jsonschema validation error into a single
// line for the API response, dropping the header line that only names the
// schema's internal URL.
func validationMessage(err error) string {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err.Error()
	}
	lines := strings.Split(ve.Error(), "\n")
	var details []string
	for _, line := range lines[1:] {
		if line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-")); line != "" {
			details = append(details, line)
		}
	}
	if len(details) == 0 {
		return lines[0]
	}
	return strings.Join(details, "; ")
}

// validateDatasetReference confirms that a body referencing a dataset, via
// both organizationId and datasetId, names a dataset that actually exists in
// that organization. A body that references no dataset passes; whether one
// is required is up to the topic's context schema.
func validateDatasetReference(ctx context.Context, fields map[string]any, fail func(int, string, error) *events.APIGatewayV2HTTPResponse) *events.APIGatewayV2HTTPResponse {
	rawOrgID, hasOrg := fields["organizationId"]
	rawDatasetID, hasDataset := fields["datasetId"]
	if !hasDataset {
		return nil
	}
	if !hasOrg {
		return fail(http.StatusBadRequest, "datasetId requires organizationId", errors.New("organizationId missing"))
	}

	orgID, err := positiveInt64(rawOrgID)
	if err != nil {
		return fail(http.StatusBadRequest, "organizationId must be a positive integer", err)
	}
	datasetID, err := positiveInt64(rawDatasetID)
	if err != nil {
		return fail(http.StatusBadRequest, "datasetId must be a positive integer", err)
	}

	exists, err := db.DatasetExists(ctx, orgID, datasetID)
	if err != nil {
		return fail(http.StatusInternalServerError, "failed to validate dataset", err)
	}
	if !exists {
		return fail(http.StatusBadRequest, "dataset not found",
			fmt.Errorf("no dataset %d in organization %d", datasetID, orgID))
	}
	return nil
}

// positiveInt64 converts a json.Number decoded by jsonschema.UnmarshalJSON
// into a positive int64.
func positiveInt64(v any) (int64, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, fmt.Errorf("got %T, want a number", v)
	}
	i, err := n.Int64()
	if err != nil {
		return 0, err
	}
	if i <= 0 {
		return 0, fmt.Errorf("got %d", i)
	}
	return i, nil
}
