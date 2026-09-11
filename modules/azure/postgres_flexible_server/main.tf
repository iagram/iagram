variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "pg_version" {
  type    = string
  default = "16"
}
variable "sku_name" {
  type    = string
  default = "B_Standard_B1ms"
}
variable "storage_mb" {
  type    = number
  default = 32768
}
variable "admin_username" {
  type    = string
  default = "pgadmin"
}
variable "allowed_ip_addresses" {
  type        = list(string)
  description = "Public IPs allowed through the server firewall (from connects_to arrows)."
  default     = []
}


resource "random_password" "admin" {
  length  = 24
  special = false
}

resource "azurerm_postgresql_flexible_server" "this" {
  name                          = replace(lower(var.name), "/[^a-z0-9-]/", "-")
  resource_group_name           = var.resource_group_name
  location                      = var.location
  version                       = var.pg_version
  sku_name                      = var.sku_name
  storage_mb                    = var.storage_mb
  administrator_login           = var.admin_username
  administrator_password        = random_password.admin.result
  public_network_access_enabled = true
  backup_retention_days         = 7
  zone                          = "1"
  tags                          = var.tags
}

resource "azurerm_postgresql_flexible_server_firewall_rule" "clients" {
  count            = length(var.allowed_ip_addresses)
  name             = "client-${count.index}"
  server_id        = azurerm_postgresql_flexible_server.this.id
  start_ip_address = var.allowed_ip_addresses[count.index]
  end_ip_address   = var.allowed_ip_addresses[count.index]
}

output "fqdn" { value = azurerm_postgresql_flexible_server.this.fqdn }
output "server_id" { value = azurerm_postgresql_flexible_server.this.id }
output "admin_password" {
  value     = random_password.admin.result
  sensitive = true
}
