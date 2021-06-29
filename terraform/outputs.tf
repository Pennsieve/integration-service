##############################################
# PLATFORM EVENTS SNS INTEGRATION            #
##############################################

output "integration_events_sns_topic_arn" {
  value = aws_sns_topic.integration_events_sns_topic.arn
}

output "integration_events_sns_topic_name" {
  value = aws_sns_topic.integration_events_sns_topic.name
}