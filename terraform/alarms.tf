# T1 CloudWatch alarms (EPIC 868m2zvjt; standard sets from
# pennsieve-infra-dashboard/docs/alarm-coverage-plan.md). The
# event-integration DLQ depth alarm already exists in cloudwatch.tf and
# stays there — this adds the lambda sets and the work-queue age alarm.
# No alarm_actions yet.
module "service_alarms" {
  source = "git@github.com:Pennsieve/terraform-modules.git//service-alarms"

  environment_name = var.environment_name
  service_name     = var.service_name

  lambdas = {
    event-consumer = {
      function_name   = aws_lambda_function.event_integration_consumer_lambda.function_name
      timeout_seconds = aws_lambda_function.event_integration_consumer_lambda.timeout
    }
    notification = {
      function_name   = aws_lambda_function.notification_lambda.function_name
      timeout_seconds = aws_lambda_function.notification_lambda.timeout
    }
    webhook-receiver = {
      function_name   = aws_lambda_function.webhook_receiver_lambda.function_name
      timeout_seconds = aws_lambda_function.webhook_receiver_lambda.timeout
    }
  }

  queues = {
    event-integration = {
      queue_name      = aws_sqs_queue.event_integration_queue.name
      max_age_seconds = 3600
    }
  }
}
