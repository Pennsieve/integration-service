resource "aws_lambda_function" "notification_lambda" {
  description   = "Notification API Lambda — serves the user subscription and notification retrieval API described in terraform/notification-service.yml"
  function_name = "${var.environment_name}-${var.service_name}-notification-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  role          = aws_iam_role.notification_lambda_role.arn
  timeout       = 30
  memory_size   = 128
  s3_bucket     = var.lambda_bucket
  s3_key        = "${var.service_name}/notification_handler/${var.service_name}-notification-${var.image_tag}.zip"

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

  depends_on = [aws_cloudwatch_log_group.notification_log_group]
}

resource "aws_cloudwatch_log_group" "notification_log_group" {
  name              = "/aws/lambda/${var.environment_name}-${var.service_name}-notification-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  retention_in_days = 30
}

resource "aws_iam_role" "notification_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-notification-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_policy" "notification_lambda_policy" {
  name = "${var.environment_name}-${var.service_name}-notification-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  policy = data.aws_iam_policy_document.notification_policy_document.json
}

data "aws_iam_policy_document" "notification_policy_document" {
  statement {
    sid    = "NotificationCloudwatch"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "NotificationVPC"
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
    sid    = "NotificationSSM"
    effect = "Allow"
    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = ["arn:aws:ssm:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:parameter/${var.environment_name}/${var.service_name}/*"]
  }

  statement {
    sid    = "NotificationSSMKMS"
    effect = "Allow"
    actions = ["kms:Decrypt", "kms:GenerateDataKey*"]
    resources = ["arn:aws:kms:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:key/alias/aws/ssm"]
  }

  statement {
    sid    = "NotificationRDS"
    effect = "Allow"
    actions = ["rds-db:connect"]
    resources = [local.rds_db_connect_arn]
  }
}

resource "aws_iam_role_policy_attachment" "notification_policy_attachment" {
  role       = aws_iam_role.notification_lambda_role.name
  policy_arn = aws_iam_policy.notification_lambda_policy.arn
}
