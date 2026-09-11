variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "network_id" { type = string }
variable "cidr" { type = string }
variable "region" { type = string }
variable "private_google_access" {
  type    = bool
  default = true
}

resource "google_compute_subnetwork" "this" {
  name                     = var.name
  network                  = var.network_id
  ip_cidr_range            = var.cidr
  region                   = var.region
  private_ip_google_access = var.private_google_access
}

output "subnetwork_id" { value = google_compute_subnetwork.this.id }
output "subnetwork_name" { value = google_compute_subnetwork.this.name }
output "region" { value = var.region }
