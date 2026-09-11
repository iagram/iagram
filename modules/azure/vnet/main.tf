variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "cidr" { type = string }

resource "azurerm_virtual_network" "this" {
  name                = var.name
  resource_group_name = var.resource_group_name
  location            = var.location
  address_space       = [var.cidr]
  tags                = var.tags
}

output "vnet_id" { value = azurerm_virtual_network.this.id }
output "vnet_name" { value = azurerm_virtual_network.this.name }
output "resource_group_name" { value = var.resource_group_name }
output "location" { value = var.location }
