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

variable "dbmigrate_service_name" {
  description = "The cloudwrap service name the dbmigrate container runs as (see Dockerfile.cloudwrap-dbmigrate); determines the SSM parameter path it reads its config from."
  default     = "integration-service-dbmigrate"
}

variable "dbmigrate_postgres_user" {
  type        = string
  description = "The Postgres user the dbmigrate container connects as to run schema migrations. Must have the grants required to create/alter schemas, since it connects directly to the master instance rather than through the RDS proxy."
}

locals {
  
  common_tags = {
    aws_account      = var.aws_account
    aws_region       = data.aws_region.current_region.name
    environment_name = var.environment_name
  }
  rds_db_connect_arn = "${replace(replace(data.terraform_remote_state.pennsieve_postgres.outputs.rds_proxy_endpoint_arn, ":rds:", ":rds-db:"), ":db-proxy:", ":dbuser:")}/${var.postgres_user}"
}
