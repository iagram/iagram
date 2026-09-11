variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "sku" {
  type    = string
  default = "standard"
}
variable "purge_protection" {
  type    = bool
  default = false
}

data "azurerm_client_config" "current" {}

locals {
  # 3-24 chars, alphanumerics and hyphens, globally unique.
  base  = substr(replace(lower(var.name), "/[^a-z0-9-]/", "-"), 0, 15)
  vault = "${local.base}-${substr(md5("${var.resource_group_name}/${var.name}"), 0, 8)}"
}

resource "azurerm_key_vault" "this" {
  name                       = local.vault
  resource_group_name        = var.resource_group_name
  location                   = var.location
  tenant_id                  = data.azurerm_client_config.current.tenant_id
  sku_name                   = var.sku
  rbac_authorization_enabled = true
  purge_protection_enabled   = var.purge_protection
  soft_delete_retention_days = 7
  tags                       = var.tags
}

output "key_vault_id" { value = azurerm_key_vault.this.id }
output "vault_uri" { value = azurerm_key_vault.this.vault_uri }
