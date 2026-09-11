variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "vnet_name" { type = string }
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "cidr" { type = string }

resource "azurerm_subnet" "this" {
  name                 = var.name
  resource_group_name  = var.resource_group_name
  virtual_network_name = var.vnet_name
  address_prefixes     = [var.cidr]
}

output "subnet_id" { value = azurerm_subnet.this.id }
output "resource_group_name" { value = var.resource_group_name }
output "location" { value = var.location }
