variable "aws_account" {}

variable "aws_region" {}

variable "environment_name" {}

variable "service_name" {}

variable "api_domain_name" {}

variable "vpc_name" {}

# Postgres
variable "pennsieve_postgres_host" {}

variable "pennsieve_postgres_db" {
  default = "pennsieve_postgres"
}

variable "lambda_bucket" {
  default = "pennsieve-cc-lambda-functions-use1"
}

variable "image_tag" {}

variable "postgres_user" {
  type = string
  description = "The username for the Postgres database. This is used to connect to the database."
}

# JWT authorizer for user-facing routes (see the auth assumption documented in
# terraform/notification-service.yml): the notification/subscription API sits
# behind the same JWT authorizer used by other Pennsieve services, which
# places the caller's Pennsieve user id in the "user_id" claim.
variable "jwt_authorizer_issuer" {
  type        = string
  description = "Issuer URL of the shared Pennsieve JWT authorizer used to validate bearer tokens on user-facing routes."
}

variable "jwt_authorizer_audience" {
  type        = list(string)
  description = "Allowed audience(s) for the shared Pennsieve JWT authorizer."
}


locals {
  
  common_tags = {
    aws_account      = var.aws_account
    aws_region       = data.aws_region.current_region.name
    environment_name = var.environment_name
  }
  rds_db_connect_arn = "${replace(replace(data.terraform_remote_state.pennsieve_postgres.outputs.rds_proxy_endpoint_arn, ":rds:", ":rds-db:"), ":db-proxy:", ":dbuser:")}/${var.postgres_user}"
}
