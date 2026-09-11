variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "network_name" { type = string }
variable "ports" {
  type    = string
  default = "22,443"
}
variable "source_ranges" {
  type    = string
  default = "0.0.0.0/0"
}

locals {
  target_tag = "fw-${var.name}"
  ports      = [for p in split(",", var.ports) : trimspace(p) if trimspace(p) != ""]
}

resource "google_compute_firewall" "this" {
  name          = var.name
  network       = var.network_name
  direction     = "INGRESS"
  source_ranges = [var.source_ranges]
  target_tags   = [local.target_tag]
  allow {
    protocol = "tcp"
    ports    = local.ports
  }
}

output "target_tag" { value = local.target_tag }
