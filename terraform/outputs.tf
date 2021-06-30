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