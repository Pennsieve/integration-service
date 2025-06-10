##############################
# EVENT-INTEGRATION-LAMBDA   #
##############################
resource "aws_iam_role" "event_integration_consumer_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-event-consumer-lambda-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

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

resource "aws_iam_policy" "event_integration_consumer_lambda_iam_policy" {
  name   = "${var.environment_name}-${var.service_name}-event-consumer-lambda-iam-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  path   = "/"
  policy = data.aws_iam_policy_document.event_integration_consumer_lambda_iam_policy_document.json
}

data "aws_iam_policy_document" "event_integration_consumer_lambda_iam_policy_document" {
  statement {
    sid    = "EventIntegrationConsumerPermissions"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutDestination",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
      "ec2:CreateNetworkInterface",
      "ec2:DescribeNetworkInterfaces",
      "ec2:DeleteNetworkInterface",
      "ec2:AssignPrivateIpAddresses",
      "ec2:UnassignPrivateIpAddresses"
    ]
    resources = ["*"]
  }

  statement {
    sid       = "KMSDecryptPermissions"
    effect    = "Allow"
    actions   = ["kms:Decrypt", "kms:GenerateDataKey*"]
    resources = [aws_kms_alias.event-integration_sqs_kms_key_alias.arn,
      "arn:aws:kms:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:key/alias/aws/ssm"]
  }

  statement {
    sid    = "SSMPermissions"
    effect = "Allow"

    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]

    resources = ["arn:aws:ssm:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:parameter/${var.environment_name}/${var.service_name}/*"]
  }

  statement {
    sid    = "LambdaReadFromEventsPermission"
    effect = "Allow"

    actions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes",
      "sqs:GetQueueUrl"
    ]

    resources = [
      aws_sqs_queue.event_integration_queue.arn,
      "${aws_sqs_queue.event_integration_queue.arn}/*",
    ]
  }

  statement {
    sid = "AllowAccessToQueueKMSkey"
    effect = "Allow"
    actions = [
      "kms:Decrypt",
      "kms:Encrypt",
      "kms:GenerateDataKey"
    ]
    resources = [
      aws_kms_key.event_integration_sqs_kms_key.arn,
      aws_kms_alias.event-integration_sqs_kms_key_alias.arn
    ]
  }
}

resource "aws_iam_role_policy_attachment" "event_integration_consumer_lambda_iam_policy_attachment" {
  role       = aws_iam_role.event_integration_consumer_lambda_role.name
  policy_arn = aws_iam_policy.event_integration_consumer_lambda_iam_policy.arn
}

## Integration Service
resource "aws_iam_role" "integration_service_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-integration-service-lambda-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

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

resource "aws_lambda_permission" "integration_service_api_api_gateway_lambda_permission" {
  statement_id  = "AllowExecutionFromGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.integration_service_lambda.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn = "${data.terraform_remote_state.api_gateway.outputs.execution_arn}/*"
}

resource "aws_iam_policy" "integration_service_lambda_iam_policy" {
  name   = "${var.environment_name}-${var.service_name}-integration-service-lambda-iam-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  path   = "/"
  policy = data.aws_iam_policy_document.integration_service_lambda_iam_policy_document.json
}

data "aws_iam_policy_document" "integration_service_lambda_iam_policy_document" {
  statement {
    sid    = "EventIntegrationConsumerPermissions"
    effect = "Allow"
    actions = [
      "rds-db:connect",
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutDestination",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
      "ec2:CreateNetworkInterface",
      "ec2:DescribeNetworkInterfaces",
      "ec2:DeleteNetworkInterface",
      "ec2:AssignPrivateIpAddresses",
      "ec2:UnassignPrivateIpAddresses"
    ]
    resources = ["*"]
  }

  statement {
    sid    = "PassRole"
    effect = "Allow"
    actions = [
      "iam:PassRole",
    ]
    resources = [
      "*"
    ]
  }

  statement {
    sid = "LambdaAccessToDynamoDB"
    effect = "Allow"

    actions = [
      "dynamodb:BatchGetItem",
      "dynamodb:GetItem",
      "dynamodb:Query",
      "dynamodb:Scan",
      "dynamodb:BatchWriteItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem"
    ]

    resources = [
      aws_dynamodb_table.integrations_table.arn,
      "${aws_dynamodb_table.integrations_table.arn}/*",
      aws_dynamodb_table.workflows_table.arn,
      "${aws_dynamodb_table.workflows_table.arn}/*",
      aws_dynamodb_table.workflow_instance_status_table.arn,
      "${aws_dynamodb_table.workflow_instance_status_table.arn}/*",
      aws_dynamodb_table.workflow_instance_processor_status_table.arn,
      "${aws_dynamodb_table.workflow_instance_processor_status_table.arn}/*"
    ]

  }

}

resource "aws_iam_role_policy_attachment" "integration_service_lambda_iam_policy_attachment" {
  role       = aws_iam_role.integration_service_lambda_role.name
  policy_arn = aws_iam_policy.integration_service_lambda_iam_policy.arn
}

##########################
# SQS Queue Key Policies #
##########################

data "aws_iam_policy_document" "event_integration_queue_kms_key_policy_document" {
  statement {
    sid       = "Enable IAM User Permissions"
    effect    = "Allow"
    actions   = ["kms:*"]
    resources = ["*"]

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.terraform_remote_state.account.outputs.aws_account_id}:root"]
    }
  }

  statement {
    sid       = "Allow specific lambda to use this key"
    effect    = "Allow"

    actions    = [
      "kms:Encrypt",
      "kms:Decrypt",
      "kms:GenerateDataKey*"
    ]

    principals {
      type = "AWS"
      identifiers   = [aws_iam_role.event_integration_consumer_lambda_role.arn]
    }

  }

  statement {
    sid    = "Enable SNS "
    effect = "Allow"

    actions = [
      "kms:GenerateDataKey",
      "kms:Decrypt",
    ]

    resources = ["*"]

    principals {
      type        = "Service"
      identifiers = ["sns.amazonaws.com", "events.amazonaws.com"]
    }
  }
}
