variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "network_id" { type = string }
variable "subnetwork_id" { type = string }
variable "region" { type = string }
variable "autopilot" {
  type    = bool
  default = true
}
variable "release_channel" {
  type    = string
  default = "REGULAR"
}
variable "node_count" {
  type    = number
  default = 1
}
variable "machine_type" {
  type    = string
  default = "e2-standard-2"
}

resource "google_container_cluster" "this" {
  name                = var.name
  location            = var.region
  network             = var.network_id
  subnetwork          = var.subnetwork_id
  enable_autopilot    = var.autopilot ? true : null
  deletion_protection = false
  resource_labels     = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }

  # Standard mode: replace the default pool with a managed one below.
  remove_default_node_pool = var.autopilot ? null : true
  initial_node_count       = var.autopilot ? null : 1

  release_channel {
    channel = var.release_channel
  }

  ip_allocation_policy {}

  workload_identity_config {
    workload_pool = "${data.google_project.current.project_id}.svc.id.goog"
  }
}

data "google_project" "current" {}

resource "google_container_node_pool" "default" {
  count      = var.autopilot ? 0 : 1
  name       = "${var.name}-default"
  cluster    = google_container_cluster.this.id
  node_count = var.node_count

  node_config {
    machine_type = var.machine_type
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
    shielded_instance_config {
      enable_secure_boot = true
    }
  }
}

output "cluster_name" { value = google_container_cluster.this.name }
output "cluster_id" { value = google_container_cluster.this.id }
output "endpoint" { value = google_container_cluster.this.endpoint }
