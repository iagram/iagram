variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "routing_mode" {
  type    = string
  default = "REGIONAL"
}

resource "google_compute_network" "this" {
  name                    = var.name
  auto_create_subnetworks = false
  routing_mode            = var.routing_mode
}

# Private services access so Cloud SQL (and friends) can attach with a private IP.
resource "google_compute_global_address" "private_services" {
  name          = "${var.name}-private-services"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.this.id
}

resource "google_service_networking_connection" "private_services" {
  network                 = google_compute_network.this.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_services.name]
}

output "network_id" { value = google_compute_network.this.id }
output "network_name" { value = google_compute_network.this.name }
output "network_self_link" { value = google_compute_network.this.self_link }
output "private_service_connection" { value = google_service_networking_connection.private_services.id }
