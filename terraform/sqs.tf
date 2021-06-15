## Event Queue which contains all events that are generated on the Pennsieve Platform
resource "aws_sqs_queue" "event_integration_queue" {
  name                       = "${var.environment_name}-event-integration-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
  kms_master_key_id          = "alias/${var.environment_name}-event-integration-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  redrive_policy             = "{\"deadLetterTargetArn\":\"${aws_sqs_queue.event_integration_deadletter_queue.arn}\",\"maxReceiveCount\":3}"
}

resource "aws_sqs_queue" "event_integration_deadletter_queue" {
  name                       = "${var.environment_name}-event-integration-deadletter-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 1209600
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
  kms_master_key_id          = "alias/${var.environment_name}-event-integration-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
}

# Mapping SQS Source to Lambda Function
resource "aws_lambda_event_source_mapping" "event_integration_source_mapping" {
  event_source_arn = aws_sqs_queue.event_integration_queue.arn
  function_name    = aws_lambda_function.event_integration_consumer_lambda.arn
  batch_size = 10
  maximum_batching_window_in_seconds = 30
}

## Event Queue that contains Event/Webhook messages that will be consumed by lambda and pushed to integrations.
## Use the same KMS key as event-integration
resource "aws_sqs_queue" "webhook_integration_queue" {
  name                       = "${var.environment_name}-webhook-integration-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
  kms_master_key_id          = "alias/${var.environment_name}-event-integration-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  redrive_policy             = "{\"deadLetterTargetArn\":\"${aws_sqs_queue.webhook_integration_deadletter_queue.arn}\",\"maxReceiveCount\":3}"
}

resource "aws_sqs_queue" "webhook_integration_deadletter_queue" {
  name                       = "${var.environment_name}-webhook-integration-deadletter-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  message_retention_seconds  = 1209600
  receive_wait_time_seconds  = 20
  visibility_timeout_seconds = 3600
  kms_master_key_id          = "alias/${var.environment_name}-event-integration-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
}

# Mapping SQS Source to Lambda Function
resource "aws_lambda_event_source_mapping" "webhook_integration_source_mapping" {
  event_source_arn = aws_sqs_queue.webhook_integration_queue.arn
  function_name    = aws_lambda_function.webhook_integration_consumer_lambda.arn
  batch_size = 100
  maximum_batching_window_in_seconds = 30
}