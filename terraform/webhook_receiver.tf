##
## Lambda Function that receives webhook POST/PUT/PATCH/DELETE requests for testing.
resource "aws_lambda_function" "webhook_receiver_lambda" {
  description   = "Webhook receiver Lambda — stores incoming webhook payloads in webhooks.messages for integration testing"
  function_name = "${var.environment_name}-${var.service_name}-webhook-receiver-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  role          = aws_iam_role.webhook_receiver_lambda_role.arn
  timeout       = 30
  memory_size   = 128
  s3_bucket     = var.lambda_bucket
  s3_key        = "${var.service_name}/webhook_handler/${var.service_name}-${var.image_tag}.zip"

  vpc_config {
    subnet_ids         = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)
    security_group_ids = [data.terraform_remote_state.platform_infrastructure.outputs.integration_service_security_group_id]
  }

  environment {
    variables = {
      ENV              = var.environment_name
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name
    }
  }
}

## Lambda Function URL — publicly reachable HTTPS endpoint, no auth for testing purposes.
resource "aws_lambda_function_url" "webhook_receiver_url" {
  function_name      = aws_lambda_function.webhook_receiver_lambda.function_name
  authorization_type = "NONE"

  cors {
    allow_origins = ["*"]
    allow_methods = ["POST", "PUT", "PATCH", "DELETE"]
  }
}

##
## IAM Role
resource "aws_iam_role" "webhook_receiver_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-webhook-receiver-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_policy" "webhook_receiver_lambda_policy" {
  name = "${var.environment_name}-${var.service_name}-webhook-receiver-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  policy = data.aws_iam_policy_document.webhook_receiver_policy_document.json
}

data "aws_iam_policy_document" "webhook_receiver_policy_document" {
  statement {
    sid    = "WebhookReceiverCloudwatch"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "WebhookReceiverVPC"
    effect = "Allow"
    actions = [
      "ec2:CreateNetworkInterface",
      "ec2:DescribeNetworkInterfaces",
      "ec2:DeleteNetworkInterface",
      "ec2:AssignPrivateIpAddresses",
      "ec2:UnassignPrivateIpAddresses",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "WebhookReceiverSSM"
    effect = "Allow"
    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = ["arn:aws:ssm:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:parameter/${var.environment_name}/${var.service_name}/*"]
  }

  statement {
    sid    = "WebhookReceiverSSMKMS"
    effect = "Allow"
    actions = ["kms:Decrypt", "kms:GenerateDataKey*"]
    resources = ["arn:aws:kms:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:key/alias/aws/ssm"]
  }

  statement {
    sid    = "WebhookReceiverRDS"
    effect = "Allow"
    actions = ["rds-db:connect"]
    resources = [local.rds_db_connect_arn]
  }
}

resource "aws_iam_role_policy_attachment" "webhook_receiver_policy_attachment" {
  role       = aws_iam_role.webhook_receiver_lambda_role.name
  policy_arn = aws_iam_policy.webhook_receiver_lambda_policy.arn
}

## Output the Function URL so it can be used as the test webhook target.
output "webhook_receiver_url" {
  description = "HTTPS endpoint for the webhook receiver (use as the target URL in integration tests)"
  value       = aws_lambda_function_url.webhook_receiver_url.function_url
}
