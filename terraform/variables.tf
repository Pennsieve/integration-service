variable "aws_account" {}

variable "aws_region" {}

variable "environment_name" {}

variable "service_name" {}

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

locals {
  
  common_tags = {
    aws_account      = var.aws_account
    aws_region       = data.aws_region.current_region.name
    environment_name = var.environment_name
  }
}