package email

import (
	"testing"

	"github.com/Pennsieve/integration-service/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequest(t *testing.T) {
	req := NewRequest(
		models.EmailRecipient{Name: "Ada Lovelace", Email: "ada@example.com"},
		Notification{
			NotificationID: 77,
			OrganizationID: "45",
			Topic:          models.Topic{Name: "DATASET_PUBLISHED", Description: "Dataset Published in Workspace"},
			Title:          "Dataset published: Sleep EEG",
			Message:        `"Sleep EEG" (version 1) was published.`,
		})

	assert.Equal(t, "user-notification", req.MessageId)
	assert.Equal(t, "integration-service-notification-77", req.DedupeId)
	require.Len(t, req.Recipients, 1)
	assert.Equal(t, "ada@example.com", req.Recipients[0].Email)
	assert.Equal(t, "Ada Lovelace", req.Recipients[0].Name)
	assert.Equal(t, map[string]any{
		"Detail":           `"Sleep EEG" (version 1) was published.`,
		"TopicDescription": "Dataset Published in Workspace",
		"subject":          "Dataset published: Sleep EEG",
		"organizationId":   int64(45),
	}, req.Context)
}

func TestNewRequest_FallbacksAndUnbranded(t *testing.T) {
	for name, orgID := range map[string]string{"no organization": "", "non-numeric organization": "org-abc", "zero": "0"} {
		t.Run(name, func(t *testing.T) {
			req := NewRequest(
				models.EmailRecipient{Email: "ada@example.com"},
				Notification{NotificationID: 1, OrganizationID: orgID, Topic: models.Topic{Name: "UPDATE_README"}, Title: "t", Message: "m"})
			assert.Equal(t, "UPDATE_README", req.Context["TopicDescription"], "the topic name stands in for a missing description")
			assert.NotContains(t, req.Context, "organizationId", "no organization means the default, unbranded template")
		})
	}
}

func TestNewSender_UnconfiguredIsNil(t *testing.T) {
	t.Setenv(QueueURLEnv, "")
	sender, err := NewSender()
	require.NoError(t, err)
	assert.Nil(t, sender)
}
