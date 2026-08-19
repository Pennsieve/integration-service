# NOTE: authorization_type = "NONE" on the /webhook route means API Gateway
# itself performs no auth — the endpoint is reachable by anyone on the
# internet. Authorization is instead enforced by the Lambda handler, which
# requires a shared secret (see aws_ssm_parameter.webhook_shared_secret) in
# the X-Pennsieve-Webhook-Secret header, and also applies a payload size cap
# and a per-sender rate limit before touching the DB.
resource "aws_apigatewayv2_api" "integration_service_api" {
  name          = "${var.environment_name}-${var.service_name}-api-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  protocol_type = "HTTP"
  description   = "API for the Integration Service webhook receiver"

  cors_configuration {
    allow_origins = ["*"]
    allow_methods = ["GET", "POST", "PUT", "PATCH", "DELETE"]
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
  api_id                 = aws_apigatewayv2_api.integration_service_api.id
  integration_type       = "AWS_PROXY"
  connection_type        = "INTERNET"
  integration_method     = "POST"
  integration_uri        = aws_lambda_function.webhook_receiver_lambda.invoke_arn
  payload_format_version = "2.0"
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

##################################################
# Notifications API — terraform/notification-service.yml
##################################################

# Delegates to the shared Pennsieve Lambda REQUEST authorizer (see
# pennsieve-go-api's lambda/authorizer) rather than a native JWT authorizer:
# the "user_claim" that handler.NotificationHandler reads is minted by that
# Lambda after resolving the caller's Cognito identity against Postgres, not
# present in any raw Cognito token claim. Surfaced on
# req.RequestContext.Authorizer.Lambda["user_claim"].
resource "aws_apigatewayv2_authorizer" "pennsieve_lambda_authorizer" {
  api_id                            = aws_apigatewayv2_api.integration_service_api.id
  name                              = "${var.environment_name}-${var.service_name}-pennsieve-lambda-authorizer"
  authorizer_type                   = "REQUEST"
  authorizer_uri                    = data.terraform_remote_state.api_gateway.outputs.authorizer_lambda_invoke_uri
  authorizer_credentials_arn        = data.terraform_remote_state.api_gateway.outputs.authorizer_invocation_role
  authorizer_payload_format_version = "2.0"
  enable_simple_responses           = true
  authorizer_result_ttl_in_seconds  = 300
  identity_sources                  = ["$request.header.Authorization"]
}

resource "aws_apigatewayv2_integration" "notification_integration" {
  api_id                 = aws_apigatewayv2_api.integration_service_api.id
  integration_type       = "AWS_PROXY"
  connection_type        = "INTERNET"
  integration_method     = "POST"
  integration_uri        = aws_lambda_function.notification_lambda.invoke_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "notification_get_topics_route" {
  api_id             = aws_apigatewayv2_api.integration_service_api.id
  route_key          = "GET /notification/topics"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_get_subscriptions_route" {
  api_id             = aws_apigatewayv2_api.integration_service_api.id
  route_key          = "GET /notification/subscriptions"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_subscribe_route" {
  api_id             = aws_apigatewayv2_api.integration_service_api.id
  route_key          = "POST /notification/subscriptions/{topicId}"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_unsubscribe_route" {
  api_id             = aws_apigatewayv2_api.integration_service_api.id
  route_key          = "DELETE /notification/subscriptions/{subscriptionId}"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_get_topic_notifications_route" {
  api_id             = aws_apigatewayv2_api.integration_service_api.id
  route_key          = "GET /notification/{topicId}/notifications"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_lambda_permission" "notification_apigateway_permission" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.notification_lambda.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.integration_service_api.execution_arn}/*/*"
}