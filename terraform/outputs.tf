##############################################
# PLATFORM EVENTS SNS INTEGRATION            #
##############################################

output "integration_events_sns_topic_arn" {
  value = aws_sns_topic.integration_events_sns_topic.arn
}

output "integration_events_sns_topic_name" {
  value = aws_sns_topic.integration_events_sns_topic.name
}

output "integration_events_kms_key_arn" {
  value = aws_kms_key.event_integration_sqs_kms_key.arn
}

output "webhook_receiver_url" {
  description = "HTTPS endpoint for the webhook receiver (use as the target URL in integration tests)"
  value       = "https://${var.api_domain_name}/integration/webhook"
}

output "notification_api_url" {
  description = "Base HTTPS URL for the Notifications API described in terraform/notification-service.yml"
  value       = "https://${var.api_domain_name}/integration/notification"
}