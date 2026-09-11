variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "domain" { type = string }
variable "a_records" {
  type        = list(string)
  description = "\"<name>|<ip>\" entries from resolves_to arrows (the instance must have an external IP)."
  default     = []
}

locals {
  records = [for r in var.a_records : { name = split("|", r)[0], ip = split("|", r)[1] }]
}

resource "google_dns_managed_zone" "this" {
  name        = replace(lower(var.name), "/[^a-z0-9-]/", "-")
  dns_name    = "${trimsuffix(var.domain, ".")}."
  description = var.name
  visibility  = "public"
  labels      = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }
}

resource "google_dns_record_set" "a" {
  count        = length(local.records)
  managed_zone = google_dns_managed_zone.this.name
  name         = "${local.records[count.index].name}.${google_dns_managed_zone.this.dns_name}"
  type         = "A"
  ttl          = 300
  rrdatas      = [local.records[count.index].ip]
}

output "zone_name" { value = google_dns_managed_zone.this.name }
output "name_servers" { value = join(",", google_dns_managed_zone.this.name_servers) }
