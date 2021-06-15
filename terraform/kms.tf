resource "aws_kms_alias" "event-integration_sqs_kms_key_alias" {
  name          = "alias/${var.environment_name}-event-integration-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  target_key_id = aws_kms_key.event_integration_sqs_kms_key.arn
}

resource "aws_kms_key" "event_integration_sqs_kms_key" {
  description             = "${var.environment_name}-event-integration-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  deletion_window_in_days = 10
  enable_key_rotation     = true
  policy                  = data.aws_iam_policy_document.event_integration_queue_kms_key_policy_document.json
}