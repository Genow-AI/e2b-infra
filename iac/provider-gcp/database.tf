# Reuse the EXISTING platform Cloud SQL instance (referenced by name, never
# managed here) and create a dedicated e2b database + user on it, with a
# Terraform-generated password.
#
# The connection string is exposed as a SENSITIVE output (not written to Secret
# Manager here): the API reads that secret via a data source that must be
# populated BEFORE the API job deploys, so it can't be TF-written in the same
# apply. After `make apply` (Task 16) creates these, seed the secret from the
# output (plan Task 13):
#   terraform output -raw e2b_postgres_connection_string | \
#     gcloud secrets versions add e2b-postgres-connection-string \
#     --project=<project> --data-file=-
#
# `make destroy` drops only this e2b database + user, not the instance.

variable "sql_instance_name" {
  type        = string
  description = "Existing Cloud SQL instance that hosts the e2b database (reused, not created here)."
}

data "google_sql_database_instance" "e2b" {
  name    = var.sql_instance_name
  project = var.gcp_project_id
}

resource "google_sql_database" "e2b" {
  name     = "e2b"
  instance = var.sql_instance_name
  project  = var.gcp_project_id
}

resource "random_password" "e2b_db" {
  length  = 32
  special = false # keep it URL-safe for the connection string
}

resource "google_sql_user" "e2b" {
  name     = "e2b"
  instance = var.sql_instance_name
  project  = var.gcp_project_id
  password = random_password.e2b_db.result
}

output "e2b_db_password" {
  description = "Generated password for the e2b Postgres user."
  value       = random_password.e2b_db.result
  sensitive   = true
}

output "e2b_postgres_connection_string" {
  description = "Connection string to seed into the e2b-postgres-connection-string secret (plan Task 13)."
  value       = "postgresql://e2b:${random_password.e2b_db.result}@${data.google_sql_database_instance.e2b.private_ip_address}:5432/e2b?sslmode=disable"
  sensitive   = true
}
