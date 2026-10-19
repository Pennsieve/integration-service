// Package email sends notification emails through the Pennsieve
// email-service. It never talks to SES: it builds an email-service
// EmailRequest for the "user-notification" template and enqueues it on the
// email-service SQS queue with the email-service client, and the
// email-service renders the template (with the organization's branding),
// delivers it, and journals the send.
package email

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/Pennsieve/integration-service/internal/aws"
	"github.com/Pennsieve/integration-service/internal/models"
	emailclient "github.com/pennsieve/email-service/client"
)

// QueueURLEnv names the environment variable holding the email-service send
// queue's URL (its email_service_queue_url Terraform output).
const QueueURLEnv = "EMAIL_SERVICE_QUEUE_URL"

// Sender enqueues email requests. *emailclient.Client satisfies it; tests
// provide a fake.
type Sender interface {
	Send(ctx context.Context, req emailclient.EmailRequest) error
}

// NewSender returns a Sender for the queue named by QueueURLEnv, using the
// SQS client aws.InitAWS built. It returns a nil Sender and no error when
// QueueURLEnv is unset, meaning email is not configured for this
// environment (e.g. local runs and tests); callers skip email then.
func NewSender() (Sender, error) {
	queueURL := os.Getenv(QueueURLEnv)
	if queueURL == "" {
		return nil, nil
	}
	sqsClient, err := aws.SQSClient()
	if err != nil {
		return nil, fmt.Errorf("email sender: %w", err)
	}
	return emailclient.New(sqsClient, queueURL), nil
}

// Notification is the content of one notification email.
type Notification struct {
	// NotificationID is the notifications.notifications row the email is
	// for. It makes the email's dedupe id, so the email-service sends it at
	// most once however many times the request is enqueued.
	NotificationID int64
	// OrganizationID is the organization the event happened in, as the
	// event carries it ("" or non-numeric when it has none). It selects the
	// organization's branded template.
	OrganizationID string
	// Topic is what the recipient subscribed to; its description (or name,
	// when it has none) tells them why they got the email.
	Topic models.Topic
	// Title is the email's subject; Message is its body text.
	Title   string
	Message string
}

// NewRequest builds the "user-notification" email for n, addressed to to.
func NewRequest(to models.EmailRecipient, n Notification) emailclient.EmailRequest {
	topicDescription := n.Topic.Description
	if topicDescription == "" {
		topicDescription = n.Topic.Name
	}
	req := emailclient.UserNotification(
		emailclient.To{Name: to.Name, Email: to.Email},
		emailclient.UserNotificationArgs{
			Detail:           n.Message,
			TopicDescription: topicDescription,
		},
	).
		WithSubject(n.Title).
		WithDedupeId(DedupeID(n.NotificationID))
	if orgID, err := strconv.ParseInt(n.OrganizationID, 10, 64); err == nil && orgID > 0 {
		req = req.WithOrganization(orgID)
	}
	return req
}

// DedupeID is the email-service dedupe id of the email for notificationID.
// It is namespaced to this service so it can't collide with another
// producer's ids.
func DedupeID(notificationID int64) string {
	return "integration-service-notification-" + strconv.FormatInt(notificationID, 10)
}
