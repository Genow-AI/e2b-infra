# Reuse the EXISTING platform Cloud SQL instance (referenced by name, never
# managed here) and create a dedicated e2b database + user on it.
#
# The password is operator-set (not TF-generated): the API reads the connection
# string via a Secret Manager DATA SOURCE that is evaluated at PLAN time, so the
# secret must be seeded BEFORE `make plan` (plan Task 13) — which means the
# password must be known up front. Use the same value here and in that secret.
#
# `make destroy` drops only this e2b database + user, not the instance.

variable "sql_instance_name" {
  type        = string
  default     = "gcp-infrastructure-genow-euw3-sql-0000"
  description = "Existing Cloud SQL instance that hosts the e2b database (reused, not created here)."
}

variable "sql_e2b_password" {
  type        = string
  sensitive   = true
  description = "Password for the e2b Postgres user. Generate once (e.g. `openssl rand -base64 24`); use the SAME value when seeding the connection-string secret (Task 13)."
}

resource "google_sql_database" "e2b" {
  name     = "e2b"
  instance = var.sql_instance_name
  project  = var.gcp_project_id
}

resource "google_sql_user" "e2b" {
  name     = "e2b"
  instance = var.sql_instance_name
  project  = var.gcp_project_id
  password = var.sql_e2b_password
}
