variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "resource_group_name" { type = string }
variable "domain" { type = string }
variable "a_records" {
  type        = list(string)
  description = "\"<name>|<ip>\" entries from resolves_to arrows (the VM must have a public IP)."
  default     = []
}

locals {
  records = [for r in var.a_records : { name = split("|", r)[0], ip = split("|", r)[1] }]
}

resource "azurerm_dns_zone" "this" {
  name                = var.domain
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_dns_a_record" "named" {
  count               = length(local.records)
  name                = local.records[count.index].name
  zone_name           = azurerm_dns_zone.this.name
  resource_group_name = var.resource_group_name
  ttl                 = 300
  records             = [local.records[count.index].ip]
  tags                = var.tags
}

output "zone_id" { value = azurerm_dns_zone.this.id }
output "name_servers" { value = join(",", azurerm_dns_zone.this.name_servers) }
