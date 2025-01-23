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
  s3_bucket         = var.lambda_bucket
  s3_key            = "${var.service_name}/event_handler/${var.service_name}-${var.image_tag}.zip"

  vpc_config {
    subnet_ids         = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)
    security_group_ids = [data.terraform_remote_state.platform_infrastructure.outputs.integration_service_security_group_id]
  }

  environment {
    variables = {
      ENV = var.environment_name
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name,
#      WEBHOOK_SQS_QUEUE_NAME = aws_sqs_queue.webhook_integration_queue.name
    }
  }
}

## Lambda Function for Integration Service.
resource "aws_lambda_function" "integration_service_lambda" {
  description      = "Lambda Function which provides the interface for integration applications"
  function_name    = "${var.environment_name}-${var.service_name}-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler          = "bootstrap"
  runtime          = "provided.al2"
  architectures    = ["arm64"]
  role             = aws_iam_role.integration_service_lambda_role.arn
  timeout          = 300
  memory_size      = 128
  s3_bucket         = var.lambda_bucket
  s3_key            = "${var.service_name}/service/${var.service_name}-${var.image_tag}.zip"

  vpc_config {
    subnet_ids         = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)
    security_group_ids = [data.terraform_remote_state.platform_infrastructure.outputs.integration_service_security_group_id]
  }

  environment {
    variables = {
      ENV = var.environment_name
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name,
      RDS_PROXY_ENDPOINT = data.terraform_remote_state.pennsieve_postgres.outputs.rds_proxy_endpoint,
      REGION = var.aws_region,
      LOG_LEVEL = "info",
      INTEGRATIONS_TABLE = aws_dynamodb_table.integrations_table.name,
      WORKFLOWS_TABLE = aws_dynamodb_table.workflows_table.name,
      WORKFLOW_INSTANCE_STATUS_TABLE = aws_dynamodb_table.workflow_instance_status_table.name,
    }
  }
}
