# resource "aws_dynamodb_table" "integrations_table" {
#   name           = "${var.environment_name}-integrations-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#   billing_mode   = "PAY_PER_REQUEST"
#   hash_key       = "uuid"
#
#   attribute {
#     name = "uuid"
#     type = "S"
#   }
#
#   ttl {
#     attribute_name = "TimeToExist"
#     enabled        = true
#   }
#
# tags = merge(
#   local.common_tags,
#   {
#     "Name"         = "${var.environment_name}-integrations-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#     "name"         = "${var.environment_name}-integrations-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#     "service_name" = var.service_name
#   },
#   )
# }
#
# resource "aws_dynamodb_table" "workflows_table" {
#   name           = "${var.environment_name}-workflows-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#   billing_mode   = "PAY_PER_REQUEST"
#   hash_key       = "uuid"
#
#   attribute {
#     name = "uuid"
#     type = "S"
#   }
#
#   ttl {
#     attribute_name = "TimeToExist"
#     enabled        = true
#   }
#
# tags = merge(
#   local.common_tags,
#   {
#     "Name"         = "${var.environment_name}-workflows-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#     "name"         = "${var.environment_name}-workflows-table-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#     "service_name" = var.service_name
#   },
#   )
# }
#
# // DEPRECATED -- TODO delete once data is migrated to workflow_instance_processor_status_table
# resource "aws_dynamodb_table" "workflow_instance_status_table" {
#   name         = "${var.environment_name}-workflow-instance-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#   billing_mode = "PAY_PER_REQUEST"
#   hash_key     = "workflowInstanceUuid"
#   range_key    = "processorUuid#timestamp"
#
#   attribute {
#     name = "workflowInstanceUuid"
#     type = "S"
#   }
#
#   attribute {
#     name = "processorUuid#timestamp"
#     type = "S"
#   }
#
#   tags = merge(
#     local.common_tags,
#     {
#       "Name"         = "${var.environment_name}-workflow-instance-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#       "name"         = "${var.environment_name}-workflow-instance-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#       "service_name" = var.service_name
#     },
#   )
# }
#
# // this table is unfortunately necessary because the workflow instance table puts
# // the whole "workflow" (list of processors) into a JSON blob so we can't make
# // changes to individual processor statuses without rewriting the whole field
# // which both requires a read first and is not consistent (w.r.t ACID)
# // if/when the workflow instance model is refactored to separate out the processors
# // then this should be refactored away
# resource "aws_dynamodb_table" "workflow_instance_processor_status_table" {
#   name         = "${var.environment_name}-workflow-instance-processor-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#   billing_mode = "PAY_PER_REQUEST"
#   hash_key     = "workflowInstanceUuid"
#   range_key    = "processorUuid"
#
#   attribute {
#     name = "workflowInstanceUuid"
#     type = "S"
#   }
#
#   attribute {
#     name = "processorUuid"
#     type = "S"
#   }
#
#   tags = merge(
#     local.common_tags,
#     {
#       "Name"         = "${var.environment_name}-workflow-instance-processor-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#       "name"         = "${var.environment_name}-workflow-instance-processor-status-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
#       "service_name" = var.service_name
#     },
#   )
# }
