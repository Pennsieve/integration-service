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

# Event consumer skipped-record alarm. The consumer skips and logs a record
# it can't decode rather than failing the batch, so such records never reach
# the DLQ and the alarm above can't see them. This counts the skip log line
# (event_parser.SkippedRecordMarker) and alarms on a sustained rate: an
# occasional malformed record is expected and fine to skip, but a producer
# that starts emitting a bad shape skips records every period.
#
# The log group is the one Lambda creates on first invocation, which already
# exists in every deployed environment.
resource "aws_cloudwatch_log_metric_filter" "event_consumer_skipped_records" {
  name           = "${var.environment_name}-${var.service_name}-event-consumer-skipped-records"
  log_group_name = "/aws/lambda/${aws_lambda_function.event_integration_consumer_lambda.function_name}"
  pattern        = "\"SKIPPED_EVENT_RECORD\""

  metric_transformation {
    name          = "SkippedEventRecords"
    namespace     = "${var.environment_name}/${var.service_name}"
    value         = "1"
    default_value = "0"
  }
}

resource "aws_cloudwatch_metric_alarm" "event_consumer_skipped_records_alarm" {
  alarm_name                = "${var.environment_name}-${var.service_name}-event-consumer-skipped-records-alarm-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  comparison_operator       = "GreaterThanOrEqualToThreshold"
  evaluation_periods        = "3"
  datapoints_to_alarm       = "3"
  metric_name               = aws_cloudwatch_log_metric_filter.event_consumer_skipped_records.metric_transformation[0].name
  namespace                 = aws_cloudwatch_log_metric_filter.event_consumer_skipped_records.metric_transformation[0].namespace
  period                    = "300"
  statistic                 = "Sum"
  threshold                 = "5"
  alarm_description         = "Event consumer is skipping undecodable records at a sustained rate (5+ per 5 minutes for 15 minutes); search its logs for SKIPPED_EVENT_RECORD"
  insufficient_data_actions = []
  alarm_actions             = [data.terraform_remote_state.account.outputs.data_management_victor_ops_sns_topic_id]
  ok_actions                = [data.terraform_remote_state.account.outputs.data_management_victor_ops_sns_topic_id]
  treat_missing_data        = "notBreaching"
}
