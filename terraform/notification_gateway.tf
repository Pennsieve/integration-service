##################################################
# Notifications API — terraform/notification-service.yml
##################################################

# The notifications API has its own HTTP API gateway, separate from
# integration_service_api (gateway.tf), which now serves only the /webhook
# receiver. It is mapped onto the shared API domain under the "notification"
# key, so its routes are declared without that prefix: "GET /topics" here is
# reached at https://<api_domain_name>/notification/topics.
resource "aws_apigatewayv2_api" "notification_service_api" {
  name          = "${var.environment_name}-${var.service_name}-notification-api-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  protocol_type = "HTTP"
  description   = "API for the Integration Service notifications and subscriptions"

  # Browsers only send Authorization on a cross-origin call if the preflight
  # names it in Access-Control-Allow-Headers. Leaving allow_headers unset made
  # every authenticated call from the web app fail preflight.
  cors_configuration {
    allow_origins     = local.cors_allowed_origins
    allow_methods     = ["OPTIONS", "GET", "POST", "PATCH"]
    allow_headers     = ["*"]
    allow_credentials = true
    expose_headers    = ["*"]
    max_age           = 300
  }
}

resource "aws_apigatewayv2_stage" "notification_service_api_stage" {
  api_id      = aws_apigatewayv2_api.notification_service_api.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.notification_service_api_gateway_log_group.arn

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

resource "aws_cloudwatch_log_group" "notification_service_api_gateway_log_group" {
  name              = "${var.environment_name}/${var.service_name}/notification-api-gateway"
  retention_in_days = 30
}

resource "aws_apigatewayv2_api_mapping" "notification_service_api_map" {
  api_id          = aws_apigatewayv2_api.notification_service_api.id
  domain_name     = var.api_domain_name
  stage           = aws_apigatewayv2_stage.notification_service_api_stage.id
  api_mapping_key = "notification"
}

# Delegates to the shared Pennsieve Lambda REQUEST authorizer (see
# pennsieve-go-api's lambda/authorizer) rather than a native JWT authorizer:
# the "user_claim" that handler.NotificationHandler reads is minted by that
# Lambda after resolving the caller's Cognito identity against Postgres, not
# present in any raw Cognito token claim. Surfaced on
# req.RequestContext.Authorizer.Lambda["user_claim"].
resource "aws_apigatewayv2_authorizer" "pennsieve_lambda_authorizer" {
  api_id                            = aws_apigatewayv2_api.notification_service_api.id
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
  api_id                 = aws_apigatewayv2_api.notification_service_api.id
  integration_type       = "AWS_PROXY"
  connection_type        = "INTERNET"
  integration_method     = "POST"
  integration_uri        = aws_lambda_function.notification_lambda.invoke_arn
  payload_format_version = "2.0"
}

# Route naming: plural paths name a collection (GET /topics, GET
# /subscriptions, GET /messages); singular paths name one resource by id
# (/subscription/{subscriptionId}, /user/{userId}). The route keys here must
# match the route* constants in internal/handler/notification_handler.go,
# which dispatches on req.RouteKey.
resource "aws_apigatewayv2_route" "notification_get_topics_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "GET /topics"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_get_subscriptions_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "GET /subscriptions"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_subscribe_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "POST /topic/{topicId}/subscription"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

# Subscriptions are turned off by disabling them rather than deleting them,
# so the notification history posted to them is preserved. There is no
# DELETE route.
resource "aws_apigatewayv2_route" "notification_update_subscription_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "PATCH /subscription/{subscriptionId}"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_get_messages_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "GET /messages"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

# The notification preferences routes. The handler additionally checks that
# {userId} is the caller's own id and returns 403 otherwise; the authorizer
# only establishes who the caller is, not which records they may touch.
#
# GET returns the full preferences resource (email/push opt-ins plus
# notificationsLastSeen); POST fully replaces the email/push opt-ins; PATCH
# is a partial update of just notificationsLastSeen, split out because it is
# written far more often (every notifications-UI open) than the other two.
resource "aws_apigatewayv2_route" "notification_get_user_last_seen_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "GET /user/{userId}"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_set_user_last_seen_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "POST /user/{userId}"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_apigatewayv2_route" "notification_update_user_last_seen_route" {
  api_id             = aws_apigatewayv2_api.notification_service_api.id
  route_key          = "PATCH /user/{userId}"
  target             = "integrations/${aws_apigatewayv2_integration.notification_integration.id}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.pennsieve_lambda_authorizer.id
}

resource "aws_lambda_permission" "notification_apigateway_permission" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.notification_lambda.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.notification_service_api.execution_arn}/*/*"
}
