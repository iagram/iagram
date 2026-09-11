variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "tier" {
  type    = string
  default = "Standard"
}
variable "replication" {
  type    = string
  default = "LRS"
}
variable "versioning" {
  type    = bool
  default = false
}

locals {
  # 3-24 lowercase alphanumerics, globally unique: name prefix + stable hash.
  base    = substr(replace(lower(var.name), "/[^a-z0-9]/", ""), 0, 16)
  account = "${local.base}${substr(md5("${var.resource_group_name}/${var.name}"), 0, 24 - length(local.base))}"
}

resource "azurerm_storage_account" "this" {
  name                            = local.account
  resource_group_name             = var.resource_group_name
  location                        = var.location
  account_tier                    = var.tier
  account_replication_type        = var.replication
  min_tls_version                 = "TLS1_2"
  allow_nested_items_to_be_public = false
  tags                            = var.tags

  blob_properties {
    versioning_enabled = var.versioning
  }
}

output "storage_account_id" { value = azurerm_storage_account.this.id }
output "storage_account_name" { value = azurerm_storage_account.this.name }
output "primary_blob_endpoint" { value = azurerm_storage_account.this.primary_blob_endpoint }
