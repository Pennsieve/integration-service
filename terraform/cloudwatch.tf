# Platform Events SQS Cloudwatch DLQ Alarm
resource "aws_cloudwatch_metric_alarm" "event_integration_dlq_cloudwatch_metric_alarm" {
  alarm_name                = "${var.environment_name}-event-integration-deadletter-queue-alarm-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  comparison_operator       = "GreaterThanOrEqualToThreshold"
  evaluation_periods        = "1"
  metric_name               = "ApproximateNumberOfMessagesVisible"
  namespace                 = "AWS/SQS"
  period                    = "60"
  statistic                 = "Average"
  threshold                 = "1"
  alarm_description         = "This metric monitors SQS DLQ for messages"
  insufficient_data_actions = []
  alarm_actions             = [data.terraform_remote_state.account.outputs.data_management_victor_ops_sns_topic_id]
  ok_actions                = [data.terraform_remote_state.account.outputs.data_management_victor_ops_sns_topic_id]
  treat_missing_data        = "ignore"

  dimensions = {
    QueueName = aws_sqs_queue.event_integration_deadletter_queue.name
  }
}