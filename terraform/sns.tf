resource "aws_sns_topic" "integration_events_sns_topic" {
  name = "${var.environment_name}-integration-events-sns-topic"
  kms_master_key_id = "alias/${var.environment_name}-event-integration-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
}

resource "aws_sns_topic_subscription" "integration_events_sqs_target" {
  topic_arn = aws_sns_topic.integration_events_sns_topic.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.event_integration_queue.arn
}

