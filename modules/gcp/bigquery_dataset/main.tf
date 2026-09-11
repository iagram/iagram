variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "location" {
  type    = string
  default = "EU"
}
variable "table_expiration_days" {
  type    = number
  default = 0
}

resource "google_bigquery_dataset" "this" {
  dataset_id                  = replace(var.name, "/[^A-Za-z0-9_]/", "_")
  location                    = var.location
  default_table_expiration_ms = var.table_expiration_days > 0 ? var.table_expiration_days * 86400000 : null
  labels                      = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }
}

output "dataset_id" { value = google_bigquery_dataset.this.dataset_id }
