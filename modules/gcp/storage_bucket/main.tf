variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "location" {
  type    = string
  default = "EU"
}
variable "versioning" {
  type    = bool
  default = false
}
variable "storage_class" {
  type    = string
  default = "STANDARD"
}

data "google_project" "current" {}

resource "google_storage_bucket" "this" {
  # Bucket names are global: suffix with the project number.
  name                        = substr("${replace(lower(var.name), "/[^a-z0-9-]/", "-")}-${data.google_project.current.number}", 0, 63)
  location                    = var.location
  storage_class               = var.storage_class
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
  labels                      = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }

  versioning {
    enabled = var.versioning
  }
}

output "bucket_name" { value = google_storage_bucket.this.name }
output "bucket_url" { value = google_storage_bucket.this.url }
