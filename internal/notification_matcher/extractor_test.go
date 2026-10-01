package notification_matcher

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateReadmeContext is the context JSON Schema of the UPDATE_README topic.
const updateReadmeContext = `{
	"$schema": "http://json-schema.org",
	"properties": {
		"datasetId": {"type": "integer"},
		"organizationId": {"type": "integer"}
	},
	"required": ["organizationId", "datasetId"],
	"title": "UpdateReadmeTopicConfiguration",
	"type": "object"
}`

var updateReadme = models.Topic{TopicID: 4, Name: "UPDATE_README", Context: json.RawMessage(updateReadmeContext)}

// decodeEvent decodes an event the way event_parser does, so the tests see
// the same field forms the producer sends.
func decodeEvent(t *testing.T, raw string) models.EventMessage {
	t.Helper()
	var e models.EventMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &e))
	return e
}

func TestExtractContext_ConvertsStringIdsToIntegers(t *testing.T) {
	// The producer sends ids as strings; subscriptions store them as
	// integers, and JSONB containment compares types strictly.
	event := decodeEvent(t, `{"organizationId":"45","datasetId":"123","eventType":"UPDATE_README"}`)

	extracted, err := ExtractContext(updateReadme, event)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"organizationId": int64(45), "datasetId": int64(123)}, extracted)

	b, err := json.Marshal(extracted)
	require.NoError(t, err)
	assert.JSONEq(t, `{"organizationId":45,"datasetId":123}`, string(b))
}

func TestExtractContext_OnlyRequiredFields(t *testing.T) {
	// datasetId is optional here, so it must not be extracted even though
	// the event carries it: subscriptions without it would never match.
	topic := models.Topic{TopicID: 5, Name: "ORG_EVENT", Context: json.RawMessage(`{
		"type": "object",
		"properties": {"organizationId": {"type": "integer"}, "datasetId": {"type": "integer"}},
		"required": ["organizationId"]
	}`)}
	event := decodeEvent(t, `{"organizationId":"45","datasetId":"123"}`)

	extracted, err := ExtractContext(topic, event)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"organizationId": int64(45)}, extracted)
}

func TestExtractContext_TopicWithoutContextExtractsEmptyObject(t *testing.T) {
	event := decodeEvent(t, `{"eventType":"SOMETHING"}`)
	for name, topicContext := range map[string]json.RawMessage{
		"nil":         nil,
		"null":        json.RawMessage(`null`),
		"no required": json.RawMessage(`{"type":"object"}`),
	} {
		t.Run(name, func(t *testing.T) {
			extracted, err := ExtractContext(models.Topic{TopicID: 6, Context: topicContext}, event)
			require.NoError(t, err)
			assert.Empty(t, extracted)

			b, err := json.Marshal(extracted)
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(b))
		})
	}
}

func TestExtractContext_BadFields(t *testing.T) {
	cases := map[string]string{
		"missing datasetId":          `{"organizationId":"45","eventType":"UPDATE_README"}`,
		"null datasetId":             `{"organizationId":"45","datasetId":null}`,
		"missing organizationId":     `{"datasetId":"123"}`,
		"non-numeric organizationId": `{"organizationId":"org1","datasetId":"123"}`,
		"zero organizationId":        `{"organizationId":"0","datasetId":"123"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ExtractContext(updateReadme, decodeEvent(t, raw))
			assert.ErrorIs(t, err, ErrBadField)
		})
	}

	// Decoding rejects a non-positive datasetId, but the extractor checks
	// it too for events built some other way.
	t.Run("negative datasetId", func(t *testing.T) {
		_, err := ExtractContext(updateReadme, models.EventMessage{OrgID: "45", DatasetID: new(-1)})
		assert.ErrorIs(t, err, ErrBadField)
	})
}

func TestExtractContext_RequiredFieldEventsDontCarry(t *testing.T) {
	topic := models.Topic{TopicID: 7, Name: "PACKAGE_EVENT", Context: json.RawMessage(`{"required":["packageId"]}`)}
	_, err := ExtractContext(topic, decodeEvent(t, `{"organizationId":"45","datasetId":"123"}`))
	assert.ErrorIs(t, err, ErrBadField)
}

func TestExtractContext_InvalidTopicContextIsNotABadField(t *testing.T) {
	topic := models.Topic{TopicID: 8, Context: json.RawMessage(`{"required":"datasetId"}`)}
	_, err := ExtractContext(topic, decodeEvent(t, `{"organizationId":"45","datasetId":"123"}`))
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrBadField)
}

func TestMatch_QueriesContainmentWithIntegerContext(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	mock.ExpectQuery(regexp.QuoteMeta("WHERE topic_id = $1 AND context @> $2::jsonb")).
		WithArgs(int64(4), []byte(`{"datasetId":123,"organizationId":45}`)).
		WillReturnRows(sqlmock.NewRows([]string{"subscription_id", "user_id", "topic_id", "context", "enabled", "created_at"}).
			AddRow(int64(10), int64(7), int64(4), []byte(`{"organizationId":45,"datasetId":123}`), true, time.Now()))

	event := decodeEvent(t, `{"organizationId":"45","datasetId":"123","eventType":"UPDATE_README"}`)
	subs, err := Match(context.Background(), updateReadme, event)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, int64(10), subs[0].SubscriptionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMatch_BadFieldSkipsTheQuery(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db.SetPoolForTest(mockDB)

	event := decodeEvent(t, `{"organizationId":"45","eventType":"UPDATE_README"}`)
	_, err = Match(context.Background(), updateReadme, event)
	assert.ErrorIs(t, err, ErrBadField)
	require.NoError(t, mock.ExpectationsWereMet(), "no query should run for an event that can't match")
}
