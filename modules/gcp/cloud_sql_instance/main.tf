variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "network_id" { type = string }
variable "private_service_connection" { type = string }
variable "database_version" {
  type    = string
  default = "POSTGRES_16"
}
variable "tier" {
  type    = string
  default = "db-f1-micro"
}
variable "region" { type = string }
variable "disk_gb" {
  type    = number
  default = 10
}
variable "high_availability" {
  type    = bool
  default = false
}
variable "client_service_accounts" {
  type        = list(string)
  description = "Service accounts granted roles/cloudsql.client (from connects_to arrows)."
  default     = []
}

data "google_project" "current" {}

resource "google_sql_database_instance" "this" {
  name                = replace(lower(var.name), "/[^a-z0-9-]/", "-")
  database_version    = var.database_version
  region              = var.region
  deletion_protection = false

  settings {
    tier              = var.tier
    disk_size         = var.disk_gb
    disk_autoresize   = true
    availability_type = var.high_availability ? "REGIONAL" : "ZONAL"
    ip_configuration {
      ipv4_enabled    = false
      private_network = var.network_id
    }
    backup_configuration {
      enabled = true
    }
    user_labels = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }
  }

  # The private-services peering must exist before a private IP instance.
  depends_on = [var.private_service_connection]
}

resource "google_project_iam_member" "clients" {
  count   = length(var.client_service_accounts)
  project = data.google_project.current.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${var.client_service_accounts[count.index]}"
}

output "connection_name" { value = google_sql_database_instance.this.connection_name }
output "private_ip" { value = google_sql_database_instance.this.private_ip_address }
output "instance_name" { value = google_sql_database_instance.this.name }
