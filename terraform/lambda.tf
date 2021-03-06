##
## Lambda Function which consumes messages from the SQS queue which contains all events.
resource "aws_lambda_function" "event_integration_consumer_lambda" {
  description      = "Lambda Function which consumes messages from the SQS queue which contains all events"
  function_name    = "${var.environment_name}-${var.service_name}-event-consumer-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler          = "event_lambda.lambda_handler"
  runtime          = "python3.8"
  role             = aws_iam_role.event_integration_consumer_lambda_role.arn
  timeout          = 60
  memory_size      = 128
  source_code_hash = data.archive_file.event_lambda_archive.output_base64sha256
  filename         = "${path.module}/event_lambda.zip"

  vpc_config {
    subnet_ids         = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)
    security_group_ids = [data.terraform_remote_state.platform_infrastructure.outputs.integration_service_security_group_id]
  }

  environment {
    variables = {
      ENV = "${var.environment_name}"
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name,
#      WEBHOOK_SQS_QUEUE_NAME = aws_sqs_queue.webhook_integration_queue.name
    }
  }
}

data "archive_file" "event_lambda_archive" {
  type        = "zip"
  source_dir  = "${path.module}/lambda/event_lambda"
  output_path = "${path.module}/event_lambda.zip"
}
#
###
### Lambda Function  which consumes messages from the webhook integration SQS queue
#resource "aws_lambda_function" "webhook_integration_consumer_lambda" {
#  description      = "Lambda Function which consumes messages from the webhook integration SQS queue"
#  function_name    = "${var.environment_name}-${var.service_name}-webhook-consumer-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#  handler          = "webhook_lambda.lambda_handler"
#  runtime          = "python3.8"
#  role             = aws_iam_role.webhook_integration_consumer_lambda_role.arn
#  timeout          = 3
#  memory_size      = 128
#  source_code_hash = data.archive_file.webhook_lambda_archive.output_base64sha256
#  filename         = "${path.module}/webhook_lambda.zip"
#
#  environment {
#    variables = {
#      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name
#    }
#  }
#}
#
#data "archive_file" "webhook_lambda_archive" {
#  type        = "zip"
#  source_dir  = "${path.module}/lambda/webhook_lambda"
#  output_path = "${path.module}/webhook_lambda.zip"
#}