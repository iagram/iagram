variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "subnetwork_id" { type = string }
variable "region" { type = string }
variable "machine_type" {
  type    = string
  default = "e2-micro"
}
variable "zone" {
  type    = string
  default = "b"
}
variable "image" {
  type    = string
  default = "debian-cloud/debian-12"
}
variable "disk_gb" {
  type    = number
  default = 20
}
variable "public_ip" {
  type    = bool
  default = false
}
variable "network_tags" {
  type    = list(string)
  default = []
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

locals {
  labels = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }
}

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

resource "google_compute_instance" "this" {
  name         = var.name
  machine_type = var.machine_type
  zone         = "${var.region}-${var.zone}"
  tags         = var.network_tags
  labels       = local.labels

  boot_disk {
    initialize_params {
      image = var.image
      size  = var.disk_gb
    }
  }

  network_interface {
    subnetwork = var.subnetwork_id
    dynamic "access_config" {
      for_each = var.public_ip ? [1] : []
      content {}
    }
  }

  service_account {
    email  = google_service_account.this.email
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot = true
  }
}

output "instance_id" { value = google_compute_instance.this.id }
output "internal_ip" { value = google_compute_instance.this.network_interface[0].network_ip }
output "external_ip" { value = var.public_ip ? google_compute_instance.this.network_interface[0].access_config[0].nat_ip : "" }
output "service_account_email" { value = google_service_account.this.email }
