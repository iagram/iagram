variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "region" { type = string }
variable "image" {
  type    = string
  default = "us-docker.pkg.dev/cloudrun/container/hello"
}
variable "cpu" {
  type    = string
  default = "1"
}
variable "memory" {
  type    = string
  default = "512Mi"
}
variable "min_instances" {
  type    = number
  default = 0
}
variable "max_instances" {
  type    = number
  default = 10
}
variable "allow_unauthenticated" {
  type    = bool
  default = false
}
variable "bucket_names" {
  type    = list(string)
  default = []
}
variable "pubsub_topics" {
  type    = list(string)
  default = []
}
variable "bigquery_datasets" {
  type    = list(string)
  default = []
}
variable "cloud_sql_connection_names" {
  type    = list(string)
  default = []
}

data "google_project" "current" {}

resource "google_service_account" "this" {
  account_id   = substr(replace(lower(var.name), "/[^a-z0-9-]/", "-"), 0, 28)
  display_name = var.name
}

resource "google_pubsub_topic_iam_member" "publisher" {
  count  = length(var.pubsub_topics)
  topic  = var.pubsub_topics[count.index]
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${google_service_account.this.email}"
}

resource "google_bigquery_dataset_iam_member" "editor" {
  count      = length(var.bigquery_datasets)
  dataset_id = var.bigquery_datasets[count.index]
  role       = "roles/bigquery.dataEditor"
  member     = "serviceAccount:${google_service_account.this.email}"
}

resource "google_storage_bucket_iam_member" "objects" {
  count  = length(var.bucket_names)
  bucket = var.bucket_names[count.index]
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.this.email}"
}

resource "google_project_iam_member" "cloudsql" {
  count   = length(var.cloud_sql_connection_names) > 0 ? 1 : 0
  project = data.google_project.current.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.this.email}"
}

resource "google_cloud_run_v2_service" "this" {
  name                = replace(lower(var.name), "/[^a-z0-9-]/", "-")
  location            = var.region
  deletion_protection = false
  labels              = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }

  template {
    service_account = google_service_account.this.email
    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }
    containers {
      image = var.image
      resources {
        limits = { cpu = var.cpu, memory = var.memory }
      }
      dynamic "volume_mounts" {
        for_each = length(var.cloud_sql_connection_names) > 0 ? [1] : []
        content {
          name       = "cloudsql"
          mount_path = "/cloudsql"
        }
      }
    }
    dynamic "volumes" {
      for_each = length(var.cloud_sql_connection_names) > 0 ? [1] : []
      content {
        name = "cloudsql"
        cloud_sql_instance {
          instances = var.cloud_sql_connection_names
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [template[0].containers[0].image] # deployed outside iagram
  }
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  count    = var.allow_unauthenticated ? 1 : 0
  name     = google_cloud_run_v2_service.this.name
  location = google_cloud_run_v2_service.this.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "service_url" { value = google_cloud_run_v2_service.this.uri }
output "service_account_email" { value = google_service_account.this.email }
