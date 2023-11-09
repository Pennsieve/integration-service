resource "aws_dynamodb_table" "integrations_table" {
  name           = "${var.environment_name}-integrations-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "uuid"
  range_key      = "applicationId"

  attribute {
    name = "uuid"
    type = "S"
  }

  attribute {
    name = "applicationId"
    type = "N"
  }

  attribute {
    name = "datasetNodeId"
    type = "S"
  }

  attribute {
    name = "packageIds"
    type = "S"
  }

  attribute {
    name = "params"
    type = "S"
  }

  attribute {
    name = "taskId"
    type = "S"
  }

  attribute {
    name = "start"
    type = "N"
  }

  attribute {
    name = "end"
    type = "N"
  }

  
  ttl {
    attribute_name = "TimeToExist"
    enabled        = false
  }

  global_secondary_index {
    name               = "ApplicationIdIndex"
    hash_key           = "applicationId"
    range_key          = "datasetNodeId"
    projection_type    = "INCLUDE"
    non_key_attributes = ["uuid"]
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