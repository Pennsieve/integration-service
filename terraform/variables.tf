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