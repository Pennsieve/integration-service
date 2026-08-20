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

// DBMIGRATE POSTGRES CONFIGURATION
// Read by the dbmigrate container via cloudwrap (see Dockerfile.cloudwrap-dbmigrate,
// which runs it with --service var.dbmigrate_service_name) so it can connect
// directly to the Postgres master instance and run schema migrations.
resource "aws_ssm_parameter" "dbmigrate_postgres_host" {
  name  = "/${var.environment_name}/${var.dbmigrate_service_name}/postgres-host"
  type  = "String"
  value = data.terraform_remote_state.pennsieve_postgres.outputs.master_fqdn
}

resource "aws_ssm_parameter" "dbmigrate_postgres_user" {
  name  = "/${var.environment_name}/${var.dbmigrate_service_name}/postgres-user"
  type  = "String"
  value = var.dbmigrate_postgres_user
}

resource "aws_ssm_parameter" "dbmigrate_postgres_password" {
  name      = "/${var.environment_name}/${var.dbmigrate_service_name}/postgres-password"
  overwrite = false
  type      = "SecureString"
  value     = "dummy"

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "dbmigrate_postgres_database" {
  name  = "/${var.environment_name}/${var.dbmigrate_service_name}/postgres-database"
  type  = "String"
  value = var.pennsieve_postgres_db
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