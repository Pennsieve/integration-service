##############################################
# PLATFORM EVENTS SNS INTEGRATION            #
##############################################

output "ncs_integration_events_sns_topic_arn" {
  value = aws_sns_topic.ncs_integration_events_sns_topic.arn
}