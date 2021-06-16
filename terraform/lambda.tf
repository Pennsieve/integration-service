##
## Lambda Function which consumes messages from the SQS queue which contains all events.
resource "aws_lambda_function" "event_integration_consumer_lambda" {
  description      = "A description"
  function_name    = "${var.environment_name}-${var.service_name}-event-integration-consumer-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler          = "event_lambda.lambda_handler"
  runtime          = "python3.8"
  role             = aws_iam_role.event_integration_consumer_lambda_role.arn
  timeout          = 3
  memory_size      = 128
  source_code_hash = data.archive_file.event_lambda_archive.output_base64sha256
  filename         = "${path.module}/event_lambda.zip"

  environment {
    variables = {
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name
    }
  }
}

data "archive_file" "event_lambda_archive" {
  type        = "zip"
  source_dir  = "${path.module}/lambda/event_lambda"
  output_path = "${path.module}/event_lambda.zip"
}

##
## Lambda Function  which consumes messages from the webhook integration SQS queue
resource "aws_lambda_function" "webhook_integration_consumer_lambda" {
  description      = "A description"
  function_name    = "${var.environment_name}-${var.service_name}-webhook-integration-consumer-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler          = "webhook_lambda.lambda_handler"
  runtime          = "python3.8"
  role             = aws_iam_role.webhook_integration_consumer_lambda_role.arn
  timeout          = 3
  memory_size      = 128
  source_code_hash = data.archive_file.webhook_lambda_archive.output_base64sha256
  filename         = "${path.module}/webhook_lambda.zip"

  environment {
    variables = {
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name
    }
  }
}

data "archive_file" "webhook_lambda_archive" {
  type        = "zip"
  source_dir  = "${path.module}/lambda/webhook_lambda"
  output_path = "${path.module}/webhook_lambda.zip"
}