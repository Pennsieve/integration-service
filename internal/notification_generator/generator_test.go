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
// workspace-wide topics (DATASET_PUBLISHED, ...) are seeded with.
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
	subscriptionColumns = []string{"subscription_id", "user_id", "topic_id", "context", "notification_channels", "enabled", "created_at"}
)

func publishedEvent(t *testing.T, orgID, category, detail string) models.EventMessage {
	t.Helper()
	raw := map[string]any{"organizationId": orgID, "eventCategory": category,
		"eventType": models.EventTypeDatasetPublished}
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
		WithArgs(models.EventTypeDatasetPublished).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(int64(11), models.EventTypeDatasetPublished, "published", enabled, time.Now(), []byte(datasetPublishedContext)))
}

func expectMatch(mock sqlmock.Sqlmock, orgID int, subscriptionIDs ...int64) {
	rows := sqlmock.NewRows(subscriptionColumns)
	for _, id := range subscriptionIDs {
		rows.AddRow(id, id+100, int64(11), []byte(`{"organizationId":45}`), nil, true, time.Now())
	}
	ctx, _ := json.Marshal(map[string]int{"organizationId": orgID})
	mock.ExpectQuery(regexp.QuoteMeta("WHERE topic_id = $1 AND context @> $2::jsonb")).
		WithArgs(int64(11), ctx).
		WillReturnRows(rows)
}

// expectPreferences registers the preferences read planDeliveries makes for
// userIDs, returning no rows so every user gets the column defaults.
func expectPreferences(mock sqlmock.Sqlmock, userIDs []int64) {
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.preferences")).
		WithArgs(pq.Array(userIDs)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "email_enabled", "push_enabled", "sms_enabled"}))
}

func TestGenerate_RecordsOneNotificationPerMatchedSubscription(t *testing.T) {
	mock := newMock(t)
	expectTopic(mock, true)
	expectMatch(mock, 45, 1, 2)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.notifications")).
		WithArgs(pq.Array([]int64{1, 2}),
			"Dataset published: Sleep EEG",
			`"Sleep EEG" (version 1) was published to Pennsieve Discover with DOI 10.26275/abcd. Dataset owner: Ada Lovelace.`,
			sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	// Neither subscription overrides its channels, so both owners'
	// preferences are read to plan the deliveries.
	expectPreferences(mock, []int64{101, 102})

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {publishedEvent(t, "45", models.EventCategoryPublishing, publishedDetail)},
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
		"45": {publishedEvent(t, "45", models.EventCategoryPublishing, publishedDetail)},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerate_LooksUpTopicOncePerBatch(t *testing.T) {
	mock := newMock(t)
	expectTopic(mock, true)
	expectMatch(mock, 45)
	expectMatch(mock, 46)

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {publishedEvent(t, "45", models.EventCategoryPublishing, publishedDetail)},
		"46": {publishedEvent(t, "46", models.EventCategoryPublishing, publishedDetail)},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerate_SkipsWithoutQuerying(t *testing.T) {
	cases := map[string]models.EventMessage{
		"wrong category":     publishedEvent(t, "45", models.EventCategoryOrganization, publishedDetail),
		"missing detail":     publishedEvent(t, "45", models.EventCategoryPublishing, ""),
		"detail without doi": publishedEvent(t, "45", models.EventCategoryPublishing, `{"datasetId":1,"datasetName":"x","publishedVersion":1}`),
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
			AddRow(int64(3), int64(7), int64(4), []byte(`{"organizationId":45,"datasetId":123}`), nil, true, time.Now()))
	body := `{"organizationId":"45","datasetId":123,"eventCategory":"METADATA","eventType":"UPDATE_README","eventDetail":{"readme":"new"}}`
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO notifications.notifications")).
		WithArgs(pq.Array([]int64{3}), "UPDATE_README", body, []byte(body)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectPreferences(mock, []int64{7})

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
	r := rendererFor(models.EventTypeDatasetPublished)
	assert.Equal(t, models.EventCategoryPublishing, r.category)

	r = rendererFor("UPDATE_README")
	assert.Empty(t, r.category, "the default renderer accepts any category")
	content, err := r.render(models.EventMessage{Type: "UPDATE_README"})
	require.NoError(t, err)
	assert.Equal(t, "UPDATE_README", content.Title)
}

func TestRenderDefault_FlatEventBody(t *testing.T) {
	// Fields the event doesn't carry are left out rather than sent empty.
	content, err := renderDefault(models.EventMessage{OrgID: "45", SubjectUserID: new(7), Category: "ORGANIZATION", Type: "ADD_TEAM_USER"})
	require.NoError(t, err)
	assert.Equal(t, "ADD_TEAM_USER", content.Title)
	assert.Equal(t, `{"organizationId":"45","subjectUserId":7,"eventCategory":"ORGANIZATION","eventType":"ADD_TEAM_USER"}`, content.Message)
	assert.JSONEq(t, content.Message, string(content.Metadata))
}

func TestGenerate_MissingTopicIsNotAnError(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("WHERE name = $1")).
		WithArgs(models.EventTypeDatasetPublished).
		WillReturnRows(sqlmock.NewRows(topicColumns))

	Generate(context.Background(), map[string][]models.EventMessage{
		"45": {
			publishedEvent(t, "45", models.EventCategoryPublishing, publishedDetail),
			publishedEvent(t, "45", models.EventCategoryPublishing, publishedDetail),
		},
	})
	require.NoError(t, mock.ExpectationsWereMet(), "a missing topic is looked up once per batch")
}

func TestGenerate_EventWithoutOrganizationSkipsMatching(t *testing.T) {
	mock := newMock(t)
	expectTopic(mock, true)

	Generate(context.Background(), map[string][]models.EventMessage{
		"": {publishedEvent(t, "", models.EventCategoryPublishing, publishedDetail)},
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPublicationRenderers(t *testing.T) {
	// Each lifecycle outcome has its own copy. A later version published
	// through the ordinary flow is still DATASET_PUBLISHED; only
	// DATASET_REVISED reports a revision.
	cases := []struct{ eventType, title, message string }{
		{models.EventTypeDatasetPublished, "Dataset published: Sleep EEG",
			`"Sleep EEG" (version 3) was published to Pennsieve Discover with DOI 10.26275/abcd.`},
		{models.EventTypeDatasetEmbargoed, "Dataset embargoed: Sleep EEG",
			`"Sleep EEG" (version 3) was published to Pennsieve Discover under embargo with DOI 10.26275/abcd.`},
		{models.EventTypeDatasetEmbargoReleased, "Dataset embargo released: Sleep EEG",
			`The embargo on "Sleep EEG" (version 3) has ended, and it is now public on Pennsieve Discover with DOI 10.26275/abcd.`},
		{models.EventTypeDatasetUnpublished, "Dataset unpublished: Sleep EEG",
			`"Sleep EEG" (DOI 10.26275/abcd) was removed from Pennsieve Discover.`},
		{models.EventTypeDatasetRevised, "Dataset revised: Sleep EEG",
			`The published metadata of "Sleep EEG" (version 3, DOI 10.26275/abcd) was revised on Pennsieve Discover.`},
	}
	detail := `{"datasetId":123,"datasetName":"Sleep EEG","publishedVersion":3,"doi":"10.26275/abcd"}`
	for _, c := range cases {
		t.Run(c.eventType, func(t *testing.T) {
			r := rendererFor(c.eventType)
			assert.Equal(t, models.EventCategoryPublishing, r.category)
			content, err := r.render(models.EventMessage{OrgID: "45", Category: models.EventCategoryPublishing,
				Type: c.eventType, Detail: json.RawMessage(detail)})
			require.NoError(t, err)
			assert.Equal(t, c.title, content.Title)
			assert.Equal(t, c.message, content.Message, "omits the owner when the detail doesn't name one")
			assert.JSONEq(t, `{"datasetId":123,"datasetNodeId":"","datasetName":"Sleep EEG","publishedVersion":3,
				"doi":"10.26275/abcd","ownerUserId":0,"ownerName":""}`, string(content.Metadata))
		})
	}
}

func TestPublicationRenderer_AddsOwnerAndStoresDetail(t *testing.T) {
	content, err := rendererFor(models.EventTypeDatasetPublished).render(
		publishedEvent(t, "45", models.EventCategoryPublishing, publishedDetail))
	require.NoError(t, err)
	assert.Equal(t, `"Sleep EEG" (version 1) was published to Pennsieve Discover with DOI 10.26275/abcd. Dataset owner: Ada Lovelace.`, content.Message)
	assert.JSONEq(t, publishedDetail, string(content.Metadata))
}

func TestPublicationRenderer_BadDetail(t *testing.T) {
	render := rendererFor(models.EventTypeDatasetPublished).render
	for name, detail := range map[string]string{
		"not an object":    `"oops"`,
		"no datasetId":     `{"datasetName":"x","publishedVersion":1,"doi":"d"}`,
		"no datasetName":   `{"datasetId":1,"publishedVersion":1,"doi":"d"}`,
		"version zero":     `{"datasetId":1,"datasetName":"x","publishedVersion":0,"doi":"d"}`,
		"string datasetId": `{"datasetId":"1","datasetName":"x","publishedVersion":1,"doi":"d"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := render(publishedEvent(t, "45", models.EventCategoryPublishing, detail))
			assert.ErrorIs(t, err, errBadDetail)
		})
	}
}

func channels(email, push, sms bool) *models.NotificationChannels {
	return &models.NotificationChannels{Email: email, Push: push, SMS: sms}
}

func TestPlanDeliveries_SubscriptionChannelsOverridePreferences(t *testing.T) {
	mock := newMock(t)
	// Only user 2's subscription lacks an override, so only user 2's
	// preferences are read; user 1's push-only preferences never apply.
	mock.ExpectQuery(regexp.QuoteMeta("FROM notifications.preferences")).
		WithArgs(pq.Array([]int64{2})).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "email_enabled", "push_enabled", "sms_enabled"}).
			AddRow(int64(2), false, true, false))

	topic := models.Topic{TopicID: 11, Enabled: true}
	deliveries, err := planDeliveries(context.Background(), topic, []models.Subscription{
		{SubscriptionID: 10, UserID: 1, Enabled: true, NotificationChannels: channels(false, false, true)},
		{SubscriptionID: 20, UserID: 2, Enabled: true},
	})
	require.NoError(t, err)
	assert.Equal(t, []delivery{
		{SubscriptionID: 10, UserID: 1, Channels: models.NotificationChannels{SMS: true}},
		{SubscriptionID: 20, UserID: 2, Channels: models.NotificationChannels{Push: true}},
	}, deliveries)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPlanDeliveries_FallsBackToPreferenceDefaults(t *testing.T) {
	mock := newMock(t)
	// A user with no preferences row gets the column defaults (email only).
	// Two subscriptions of the same user read their preferences once.
	expectPreferences(mock, []int64{5})

	topic := models.Topic{TopicID: 11, Enabled: true}
	deliveries, err := planDeliveries(context.Background(), topic, []models.Subscription{
		{SubscriptionID: 1, UserID: 5, Enabled: true},
		{SubscriptionID: 2, UserID: 5, Enabled: true},
	})
	require.NoError(t, err)
	assert.Equal(t, []delivery{
		{SubscriptionID: 1, UserID: 5, Channels: models.NotificationChannels{Email: true}},
		{SubscriptionID: 2, UserID: 5, Channels: models.NotificationChannels{Email: true}},
	}, deliveries)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPlanDeliveries_SkipsWhatShouldNotBeSent(t *testing.T) {
	mock := newMock(t)
	// Every subscription is overridden or disabled: no preferences read.

	topic := models.Topic{TopicID: 11, Enabled: true}
	deliveries, err := planDeliveries(context.Background(), topic, []models.Subscription{
		{SubscriptionID: 1, UserID: 1, Enabled: false},
		{SubscriptionID: 2, UserID: 2, Enabled: false, NotificationChannels: channels(true, true, true)},
		{SubscriptionID: 3, UserID: 3, Enabled: true, NotificationChannels: channels(false, false, false)},
	})
	require.NoError(t, err)
	assert.Empty(t, deliveries, "disabled subscriptions and all-off overrides send nothing")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPlanDeliveries_DisabledTopicSendsNothing(t *testing.T) {
	mock := newMock(t)

	topic := models.Topic{TopicID: 11, Enabled: false}
	deliveries, err := planDeliveries(context.Background(), topic, []models.Subscription{
		{SubscriptionID: 1, UserID: 1, Enabled: true},
		{SubscriptionID: 2, UserID: 2, Enabled: true, NotificationChannels: channels(true, false, false)},
	})
	require.NoError(t, err)
	assert.Empty(t, deliveries)
	require.NoError(t, mock.ExpectationsWereMet(), "no preferences are read for a disabled topic")
}
