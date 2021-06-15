## Event Queue which contains all events that are generated on the Pennsieve Platform

resource "aws_sqs_queue" "event_integration_queue" {
  name                       = "${var.environment_name}-event-integration-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
  redrive_policy             = "{\"deadLetterTargetArn\":\"${aws_sqs_queue.event_integration_deadletter_queue.arn}\",\"maxReceiveCount\":3}"
}

resource "aws_sqs_queue" "event_integration_deadletter_queue" {
  name                       = "${var.environment_name}-event-integration-deadletter-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 1209600
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
}

## Event Queue that contains Event/Webhook messages that will be consumed by lambda and pushed to integrations.

resource "aws_sqs_queue" "webhook_integration_queue" {
  name                       = "${var.environment_name}-webhook-integration-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
  redrive_policy             = "{\"deadLetterTargetArn\":\"${aws_sqs_queue.webhook_integration_deadletter_queue.arn}\",\"maxReceiveCount\":3}"
}

resource "aws_sqs_queue" "webhook_integration_deadletter_queue" {
  name                       = "${var.environment_name}-webhook-integration-deadletter-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 1209600
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
}
