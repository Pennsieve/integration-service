resource "aws_sns_topic" "ncs_integration_events_sns_topic" {
  name = "${var.environment_name}-ncs-integration-events-sns-topic"
}