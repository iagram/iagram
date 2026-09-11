variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "sku" {
  type    = string
  default = "Basic"
}
variable "admin_enabled" {
  type    = bool
  default = false
}

locals {
  # 5-50 alphanumerics, globally unique.
  base     = substr(replace(lower(var.name), "/[^a-z0-9]/", ""), 0, 30)
  registry = "${local.base}${substr(md5("${var.resource_group_name}/${var.name}"), 0, 8)}"
}

resource "azurerm_container_registry" "this" {
  name                = local.registry
  resource_group_name = var.resource_group_name
  location            = var.location
  sku                 = var.sku
  admin_enabled       = var.admin_enabled
  tags                = var.tags
}

output "registry_id" { value = azurerm_container_registry.this.id }
output "login_server" { value = azurerm_container_registry.this.login_server }
