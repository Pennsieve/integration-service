## Lambda Function which consumes messages from the SQS queue which contains all events.
resource "aws_lambda_function" "event_integration_consumer_lambda" {
  description      = "A description"
  function_name    = "${var.environment_name}-${var.service_name}-event-integration-consumer-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler          = "lambda.lambda_handler"
  runtime          = "python3.8"
  role             = aws_iam_role.cognito_custom_message_lambda_role.arn
  timeout          = 3
  memory_size      = 128
  source_code_hash = data.archive_file.lambda_archive.output_base64sha256
  filename         = "${path.module}/event_lambda.zip"

  environment {
    variables = {
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name
    }
  }
}

resource "aws_iam_role" "event_integration_consumer_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-event-integration-consumer-lambda-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

data "archive_file" "event_lambda_archive" {
  type        = "zip"
  source_dir  = "${path.module}/lambda"
  output_path = "${path.module}/event_lambda.zip"
}

resource "aws_iam_role_policy_attachment" "event_integration_consumer_lambda_iam_policy_attachment" {
  role       = aws_iam_role.event_integration_consumer_lambda_role.name
  policy_arn = aws_iam_policy.cognito_custom_message_lambda_iam_policy.arn
}

resource "aws_iam_policy" "cognito_custom_message_lambda_iam_policy" {
  name   = "${var.environment_name}-${var.service_name}-custom-message-lambda-iam-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  path   = "/"
  policy = data.aws_iam_policy_document.cognito_custom_message_lambda_iam_policy_document.json
}

data "aws_iam_policy_document" "cognito_custom_message_lambda_iam_policy_document" {
  statement {
    sid    = "CloudwatchLogPermissions"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutDestination",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
    ]
    resources = ["*"]
  }
}

## Lambda Function  which consumes messages from the webhook integration SQS queue
resource "aws_lambda_function" "webhook_integration_consumer_lambda" {
  description      = "A description"
  function_name    = "${var.environment_name}-${var.service_name}-webhook-integration-consumer-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler          = "lambda.lambda_handler"
  runtime          = "python3.8"
  role             = aws_iam_role.cognito_custom_message_lambda_role.arn
  timeout          = 3
  memory_size      = 128
  source_code_hash = data.archive_file.lambda_archive.output_base64sha256
  filename         = "${path.module}/webhook_lambda.zip"

  environment {
    variables = {
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name
    }
  }
}

data "archive_file" "webhook_lambda_archive" {
  type        = "zip"
  source_dir  = "${path.module}/lambda"
  output_path = "${path.module}/webhook_lambda.zip"
}