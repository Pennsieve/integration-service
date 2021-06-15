resource "aws_sns_topic" "integration_events_sns_topic" {
  name = "${var.environment_name}-integration-events-sns-topic"
}

resource "aws_sns_topic_subscription" "integration_events_sqs_target" {
  topic_arn = aws_sns_topic.integration_events_sns_topic.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.event_integration_queue.arn
}