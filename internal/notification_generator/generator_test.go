package notification_generator

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Pennsieve/integration-service/internal/db"
	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// datasetPublishedContext is the context JSON Schema the
// DATASET_PUBLISHED_IN_WORKSPACE topic is seeded with.
const datasetPublishedContext = `{
	"type": "object",
	"properties": {"organizationId": {"type": "integer"}},
	"required": ["organizationId"],
	"additionalProperties": false
}`

const publishedDetail = `{
	"datasetId": 123,
	"datasetNodeId": "N:dataset:abc",
	"datasetName": "Sleep EEG",
	"publishedVersion": 1,
	"doi": "10.26275/abcd",
	"ownerUserId": 7,
	"ownerName": "Ada Lovelace"
}`

var (
	topicColumns        = []string{"topic_id", "name", "description", "enabled", "created_at", "context"}
	subscriptionColumns = []string{"subscription_id", "user_id", "topic_id", "context", "enabled", "created_at"}
)

func publishedEvent(t *testing.T, orgID, category, detail string) models.EventMessage {
	t.Helper()
	raw := map[string]any{"organizationId": orgID, "eventCategory": category,
		"eventType": models.EventTypeDatasetPublishedInWorkspace}
	if detail != "" {
		raw["eventDetail"] = json.RawMessage(detail)
	}
	b, err := json.Marshal(raw)
	require.NoError(t, err)
	var e models.EventMessage
	require.NoError(t, json.Unmarshal(b, &e))
	return e
}

func newMock(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mockDB.Close() })
	db.SetPoolForTest(mockDB)
	return mock
}

func expectTopic(mock sqlmock.Sqlmock, enabled bool) {
	mock.ExpectQuery(regexp.QuoteMeta("WHERE name = $1")).
		WithArgs(models.EventTypeDatasetPublishedInWorkspace).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(int64(11), models.EventTypeDatasetPublishedInWorkspace, "published", enabled, time.Now(), []byte(datasetPublishedContext)))
}

func expectMatch(mock sqlmock.Sqlmock, orgID int, subscriptionIDs ...int64) {
	rows := sqlmock.NewRows(subscriptionColumns)
	for _, id := range subscriptionIDs {
		rows.AddRow(id, id+100, int64(11), []byte(`{"organizationId":45}`), true, time.Now())
	}
	ctx, _ := json.Marshal(map[string]int{"organizationId": orgID})
	mock.ExpectQuery(regexp.QuoteMeta("WHERE topic_id = $1 AND context @> $2::jsonb")).
		WithArgs(int64(11), ctx).
		WillReturnRows(rows)
}

func TestGenerate_RecordsOneNotificationPerMatchedSubscription(t *testing.T) {
	mock := newMock(t)
	expectTopic(mock, true)
	expectMatch(mock, 45, 1, 2)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.notifications")).
		WithArgs(pq.Array([]int64{1, 2}),
			"Dataset published: Sleep EEG",
			`"Sleep EEG" was published to Pennsieve Discover with DOI 10.26275/abcd. Dataset owner: Ada Lovelace.`,
			sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {publishedEvent(t, "45", models.EventCategoryOrganization, publishedDetail)},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerate_RecordsEvenWhenTopicDisabled(t *testing.T) {
	// Enabled flags gate sending, not recording.
	mock := newMock(t)
	expectTopic(mock, false)
	expectMatch(mock, 45, 1)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.notifications")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {publishedEvent(t, "45", models.EventCategoryOrganization, publishedDetail)},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerate_LooksUpTopicOncePerBatch(t *testing.T) {
	mock := newMock(t)
	expectTopic(mock, true)
	expectMatch(mock, 45)
	expectMatch(mock, 46)

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {publishedEvent(t, "45", models.EventCategoryOrganization, publishedDetail)},
		"46": {publishedEvent(t, "46", models.EventCategoryOrganization, publishedDetail)},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerate_SkipsWithoutQuerying(t *testing.T) {
	cases := map[string]models.EventMessage{
		"wrong category":     publishedEvent(t, "45", "PUBLISHING", publishedDetail),
		"missing detail":     publishedEvent(t, "45", models.EventCategoryOrganization, ""),
		"detail without doi": publishedEvent(t, "45", models.EventCategoryOrganization, `{"datasetId":1,"datasetName":"x","publishedVersion":1}`),
	}
	for name, event := range cases {
		t.Run(name, func(t *testing.T) {
			mock := newMock(t)
			Generate(context.Background(), map[string][]models.EventMessage{"45": {event}})
			require.NoError(t, mock.ExpectationsWereMet(), "no query should run")
		})
	}
}

// updateReadmeContext is the context JSON Schema of the hand-maintained
// UPDATE_README topic.
const updateReadmeContext = `{
	"type": "object",
	"properties": {"organizationId": {"type": "integer"}, "datasetId": {"type": "integer"}},
	"required": ["organizationId", "datasetId"]
}`

func TestGenerate_DefaultRendererForTypeWithoutOne(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("WHERE name = $1")).
		WithArgs("UPDATE_README").
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(int64(4), "UPDATE_README", nil, true, time.Now(), []byte(updateReadmeContext)))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE topic_id = $1 AND context @> $2::jsonb")).
		WithArgs(int64(4), []byte(`{"datasetId":123,"organizationId":45}`)).
		WillReturnRows(sqlmock.NewRows(subscriptionColumns).
			AddRow(int64(3), int64(7), int64(4), []byte(`{"organizationId":45,"datasetId":123}`), true, time.Now()))
	body := `{"organizationId":"45","datasetId":123,"eventCategory":"METADATA","eventType":"UPDATE_README","eventDetail":{"readme":"new"}}`
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.notifications")).
		WithArgs(pq.Array([]int64{3}), "UPDATE_README", body, []byte(body)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	var event models.EventMessage
	require.NoError(t, json.Unmarshal([]byte(body), &event))
	Generate(context.Background(), map[string][]models.EventMessage{"45": {event}})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerate_DatasetScopedPublicationUsesItsOwnTopic(t *testing.T) {
	// COMPLETE_PUBLICATION has no type-specific renderer, so it is rendered
	// by default against its own topic, never the workspace-wide one.
	mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("WHERE name = $1")).
		WithArgs("COMPLETE_PUBLICATION").
		WillReturnRows(sqlmock.NewRows(topicColumns))

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {{OrgID: "45", DatasetID: new(123), Category: "PUBLISHING", Type: "COMPLETE_PUBLICATION"}},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRendererFor(t *testing.T) {
	r := rendererFor(models.EventTypeDatasetPublishedInWorkspace)
	assert.Equal(t, models.EventCategoryOrganization, r.category)

	r = rendererFor("UPDATE_README")
	assert.Empty(t, r.category, "the default renderer accepts any category")
	content, err := r.render(models.EventMessage{Type: "UPDATE_README"})
	require.NoError(t, err)
	assert.Equal(t, "UPDATE_README", content.Title)
}

func TestRenderDefault_FlatEventBody(t *testing.T) {
	// Fields the event doesn't carry are left out rather than sent empty.
	content, err := renderDefault(models.EventMessage{OrgID: "45", Category: "ORGANIZATION", Type: "ADD_TEAM_USER"})
	require.NoError(t, err)
	assert.Equal(t, "ADD_TEAM_USER", content.Title)
	assert.Equal(t, `{"organizationId":"45","eventCategory":"ORGANIZATION","eventType":"ADD_TEAM_USER"}`, content.Message)
	assert.JSONEq(t, content.Message, string(content.Metadata))
}

func TestGenerate_MissingTopicIsNotAnError(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("WHERE name = $1")).
		WithArgs(models.EventTypeDatasetPublishedInWorkspace).
		WillReturnRows(sqlmock.NewRows(topicColumns))

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {
			publishedEvent(t, "45", models.EventCategoryOrganization, publishedDetail),
			publishedEvent(t, "45", models.EventCategoryOrganization, publishedDetail),
		},
	})
	require.NoError(t, mock.ExpectationsWereMet(), "a missing topic is looked up once per batch")
}

func TestGenerate_EventWithoutOrganizationSkipsMatching(t *testing.T) {
	mock := newMock(t)
	expectTopic(mock, true)

	Generate(context.Background(), map[string][]models.EventMessage{
		"": {publishedEvent(t, "", models.EventCategoryOrganization, publishedDetail)},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRenderDatasetPublishedInWorkspace(t *testing.T) {
	content, err := renderDatasetPublishedInWorkspace(publishedEvent(t, "45", models.EventCategoryOrganization, publishedDetail))
	require.NoError(t, err)
	assert.Equal(t, "Dataset published: Sleep EEG", content.Title)
	assert.Equal(t, `"Sleep EEG" was published to Pennsieve Discover with DOI 10.26275/abcd. Dataset owner: Ada Lovelace.`, content.Message)
	assert.JSONEq(t, publishedDetail, string(content.Metadata))
}

func TestRenderDatasetPublishedInWorkspace_Revision(t *testing.T) {
	detail := `{"datasetId":123,"datasetName":"Sleep EEG","publishedVersion":3,"doi":"10.26275/abcd"}`
	content, err := renderDatasetPublishedInWorkspace(publishedEvent(t, "45", models.EventCategoryOrganization, detail))
	require.NoError(t, err)
	assert.Equal(t, `Version 3 of "Sleep EEG" was published to Pennsieve Discover with DOI 10.26275/abcd.`, content.Message,
		"names the version, and omits the owner when it isn't given")
}

func TestRenderDatasetPublishedInWorkspace_BadDetail(t *testing.T) {
	for name, detail := range map[string]string{
		"not an object":    `"oops"`,
		"no datasetId":     `{"datasetName":"x","publishedVersion":1,"doi":"d"}`,
		"no datasetName":   `{"datasetId":1,"publishedVersion":1,"doi":"d"}`,
		"version zero":     `{"datasetId":1,"datasetName":"x","publishedVersion":0,"doi":"d"}`,
		"string datasetId": `{"datasetId":"1","datasetName":"x","publishedVersion":1,"doi":"d"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := renderDatasetPublishedInWorkspace(publishedEvent(t, "45", models.EventCategoryOrganization, detail))
			assert.ErrorIs(t, err, errBadDetail)
		})
	}
}
