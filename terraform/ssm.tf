// POSTGRES CONFIGURATION
resource "aws_ssm_parameter" "integrations_postgres_host" {
  name = "/${var.environment_name}/${var.service_name}/integrations-postgres-host"
  type = "String"
  value = var.pennsieve_postgres_host
}

resource "aws_ssm_parameter" "integrations_postgres_db" {
  name = "/${var.environment_name}/${var.service_name}/integrations-postgres-db"
  type = "String"
  value = var.pennsieve_postgres_db
}

resource "aws_ssm_parameter" "integrations_postgres_user" {
  name  = "/${var.environment_name}/${var.service_name}/integrations-postgres-user"
  type  = "String"
  value = "${var.environment_name}_${replace(var.service_name, "-", "_")}_user"
}

resource "aws_ssm_parameter" "integrations_postgres_password" {
  name      = "/${var.environment_name}/${var.service_name}/integrations-postgres-password"
  overwrite = false
  type      = "SecureString"
  value     = "dummy"

  lifecycle {
    ignore_changes = [value]
  }
}

// WEBHOOK RECEIVER CONFIGURATION
// Shared secret senders must present in the X-Pennsieve-Webhook-Secret
// header. Terraform only creates the parameter with a placeholder value;
// the real value is rotated by hand in SSM/console, same as the DB password
// above.
resource "aws_ssm_parameter" "webhook_shared_secret" {
  name      = "/${var.environment_name}/${var.service_name}/webhook-shared-secret"
  overwrite = false
  type      = "SecureString"
  value     = "dummy"

  lifecycle {
    ignore_changes = [value]
  }
}