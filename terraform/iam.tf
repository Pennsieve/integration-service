## EVENT-INTEGRATION-LAMBDA
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

resource "aws_iam_policy" "event_integration_consumer_lambda_iam_policy" {
  name   = "${var.environment_name}-${var.service_name}-event-integration-consumer-lambda-iam-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  path   = "/"
  policy = data.aws_iam_policy_document.event_integration_consumer_lambda_iam_policy_document.json
}

data "aws_iam_policy_document" "event_integration_consumer_lambda_iam_policy_document" {
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

  statement {
    sid       = "KMSDecryptPermissions"
    effect    = "Allow"
    actions   = ["kms:Decrypt"]
    resources = ["arn:aws:kms:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:key/alias/aws/ssm"]
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
    sid    = "SQSPermissions"
    effect = "Allow"

    actions = [
      "sqs:ReceiveMessage",
    ]

    resources = [
      aws_sqs_queue.event_integration_queue.arn,
      "${aws_sqs_queue.event_integration_queue.arn}/*"
    ]
  }

  statement {
    sid    = "SQSPermissions"
    effect = "Allow"

    actions = [
      "sqs:SendMessage"
    ]

    resources = [
      aws_sqs_queue.webhook_integration_queue.arn,
      "${aws_sqs_queue.webhook_integration_queue.arn}/*"
    ]
  }
}

resource "aws_iam_role_policy_attachment" "event_integration_consumer_lambda_iam_policy_attachment" {
  role       = aws_iam_role.event_integration_consumer_lambda_role.name
  policy_arn = aws_iam_policy.event_integration_consumer_lambda_iam_policy.arn
}

## WEBHOOK-INTEGRATION LAMBDA
resource "aws_iam_role" "webhook_integration_consumer_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-webhook-integration-consumer-lambda-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

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

resource "aws_iam_policy" "webhook_integration_consumer_lambda_iam_policy" {
  name   = "${var.environment_name}-${var.service_name}-webhook-integration-consumer-lambda-iam-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  path   = "/"
  policy = data.aws_iam_policy_document.event_integration_consumer_lambda_iam_policy_document.json
}

data "aws_iam_policy_document" "webhook_integration_consumer_lambda_iam_policy_document" {
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

  statement {
    sid       = "KMSDecryptPermissions"
    effect    = "Allow"
    actions   = ["kms:Decrypt"]
    resources = ["arn:aws:kms:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:key/alias/aws/ssm"]
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
    sid    = "SQSPermissions"
    effect = "Allow"

    actions = [
      "sqs:ReceiveMessage",
      "sqs:SendMessage"
    ]

    resources = [
      aws_sqs_queue.webhook_integration_queue.arn,
      "${aws_sqs_queue.webhook_integration_queue.arn}/*"
    ]
  }

}

resource "aws_iam_role_policy_attachment" "webhook_integration_consumer_lambda_iam_policy_attachment" {
  role       = aws_iam_role.webhook_integration_consumer_lambda_role.name
  policy_arn = aws_iam_policy.webhook_integration_consumer_lambda_iam_policy.arn
}

######################
# SQS Queue Policies #
######################

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

//  statement {
//    sid    = "Enable Cloudwatch Event Permissions"
//    effect = "Allow"
//
//    actions = [
//      "kms:GenerateDataKey",
//      "kms:Decrypt",
//    ]
//
//    resources = ["*"]
//
//    principals {
//      type        = "Service"
//      identifiers = ["events.amazonaws.com"]
//    }
//  }
}