resource "aws_dynamodb_table" "integrations_table" {
  name           = "${var.environment_name}-integrations-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "uuid"

  attribute {
    name = "uuid"
    type = "S"
  }
  
  ttl {
    attribute_name = "TimeToExist"
    enabled        = true
  }

tags = merge(
  local.common_tags,
  {
    "Name"         = "${var.environment_name}-integrations-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
    "name"         = "${var.environment_name}-integrations-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
    "service_name" = var.service_name
  },
  )
}

resource "aws_dynamodb_table" "workflows_table" {
  name           = "${var.environment_name}-workflows-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "uuid"

  attribute {
    name = "uuid"
    type = "S"
  }
  
  ttl {
    attribute_name = "TimeToExist"
    enabled        = true
  }

tags = merge(
  local.common_tags,
  {
    "Name"         = "${var.environment_name}-workflows-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
    "name"         = "${var.environment_name}-workflows-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
    "service_name" = var.service_name
  },
  )
}

resource "aws_dynamodb_table" "workflow_instance_status_table" {
  name         = "${var.environment_name}-workflow-instance-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "uuid"
  range_key    = "timestamp"

  attribute {
    name = "uuid"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "N"
  }

  tags = merge(
    local.common_tags,
    {
      "Name"         = "${var.environment_name}-workflow-instance-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "name"         = "${var.environment_name}-workflow-instance-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "service_name" = var.service_name
    },
  )
}
