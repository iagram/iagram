variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "inbound_ports" {
  type    = string
  default = "22,443"
}
variable "source_cidr" {
  type    = string
  default = "*"
}

locals {
  ports = [for p in split(",", var.inbound_ports) : trimspace(p) if trimspace(p) != ""]
}

resource "azurerm_network_security_group" "this" {
  name                = var.name
  resource_group_name = var.resource_group_name
  location            = var.location
  tags                = var.tags

  dynamic "security_rule" {
    for_each = { for i, p in local.ports : p => i }
    content {
      name                       = "allow-tcp-${security_rule.key}"
      priority                   = 100 + security_rule.value
      direction                  = "Inbound"
      access                     = "Allow"
      protocol                   = "Tcp"
      source_port_range          = "*"
      destination_port_range     = security_rule.key
      source_address_prefix      = var.source_cidr
      destination_address_prefix = "*"
    }
  }
}

output "nsg_id" { value = azurerm_network_security_group.this.id }
