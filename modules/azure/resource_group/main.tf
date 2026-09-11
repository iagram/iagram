variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "location" { type = string }

resource "azurerm_resource_group" "this" {
  name     = var.name
  location = var.location
  tags     = var.tags
}

output "resource_group_name" { value = azurerm_resource_group.this.name }
output "location" { value = azurerm_resource_group.this.location }
