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

  # Caps concurrent invocations so a burst of public, unauthenticated requests
  # can't exhaust account-wide Lambda concurrency (starving the SQS-buffered
  # event consumer) or flood RDS with connections.
  reserved_concurrent_executions = 5

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

  depends_on = [aws_cloudwatch_log_group.webhook_receiver_log_group]
}

resource "aws_cloudwatch_log_group" "webhook_receiver_log_group" {
  name              = "/aws/lambda/${var.environment_name}-${var.service_name}-webhook-receiver-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  retention_in_days = 30
}

# NOTE: authorization_type = "NONE" on the /webhook route is an intentional,
# temporary decision for testing — this makes the endpoint a public,
# unauthenticated write endpoint that lets anyone on the internet
# POST/PUT/PATCH/DELETE a row into webhooks.messages. This must be locked
# down (IAM auth, or at minimum a shared secret / WAF rule) before this
# pattern is used anywhere near prod.
resource "aws_apigatewayv2_api" "integration_service_api" {
  name          = "${var.environment_name}-${var.service_name}-api-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  protocol_type = "HTTP"
  description   = "API for the Integration Service webhook receiver"

  cors_configuration {
    allow_origins = ["*"]
    allow_methods = ["POST", "PUT", "PATCH", "DELETE"]
  }
}

resource "aws_apigatewayv2_stage" "integration_service_api_stage" {
  api_id      = aws_apigatewayv2_api.integration_service_api.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.integration_service_api_gateway_log_group.arn

    format = jsonencode({
      requestId               = "$context.requestId"
      sourceIp                = "$context.identity.sourceIp"
      requestTime             = "$context.requestTime"
      protocol                = "$context.protocol"
      httpMethod              = "$context.httpMethod"
      resourcePath            = "$context.resourcePath"
      routeKey                = "$context.routeKey"
      status                  = "$context.status"
      responseLength          = "$context.responseLength"
      integrationErrorMessage = "$context.integrationErrorMessage"
    })
  }
}

resource "aws_cloudwatch_log_group" "integration_service_api_gateway_log_group" {
  name              = "${var.environment_name}/${var.service_name}/integration-api-gateway"
  retention_in_days = 30
}

resource "aws_apigatewayv2_integration" "webhook_integration" {
  api_id             = aws_apigatewayv2_api.integration_service_api.id
  integration_type   = "AWS_PROXY"
  connection_type    = "INTERNET"
  integration_method = "POST"
  integration_uri    = aws_lambda_function.webhook_receiver_lambda.invoke_arn
}

resource "aws_apigatewayv2_route" "webhook_route" {
  api_id             = aws_apigatewayv2_api.integration_service_api.id
  route_key          = "ANY /webhook"
  target             = "integrations/${aws_apigatewayv2_integration.webhook_integration.id}"
  authorization_type = "NONE"
}

resource "aws_apigatewayv2_api_mapping" "integration_service_api_map" {
  api_id          = aws_apigatewayv2_api.integration_service_api.id
  domain_name     = var.api_domain_name
  stage           = aws_apigatewayv2_stage.integration_service_api_stage.id
  api_mapping_key = "integration"
}

resource "aws_lambda_permission" "webhook_receiver_apigateway_permission" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.webhook_receiver_lambda.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.integration_service_api.execution_arn}/*/*"
}

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

## Output the API Gateway endpoint so it can be used as the test webhook target.
output "webhook_receiver_url" {
  description = "HTTPS endpoint for the webhook receiver (use as the target URL in integration tests)"
  value       = "https://${var.api_domain_name}/integration/webhook"
}
