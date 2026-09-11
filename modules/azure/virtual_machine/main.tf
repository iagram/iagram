variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "subnet_id" { type = string }
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "size" {
  type    = string
  default = "Standard_B1s"
}
variable "admin_username" {
  type    = string
  default = "azureuser"
}
variable "os_disk_gb" {
  type    = number
  default = 30
}
variable "public_ip" {
  type    = bool
  default = true
}
variable "network_security_group_ids" {
  type    = list(string)
  default = []
}
variable "storage_account_ids" {
  type    = list(string)
  default = []
}
variable "key_vault_ids" {
  type    = list(string)
  default = []
}

resource "tls_private_key" "ssh" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "azurerm_public_ip" "this" {
  count               = var.public_ip ? 1 : 0
  name                = "${var.name}-pip"
  resource_group_name = var.resource_group_name
  location            = var.location
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = var.tags
}

resource "azurerm_network_interface" "this" {
  name                = "${var.name}-nic"
  resource_group_name = var.resource_group_name
  location            = var.location
  tags                = var.tags

  ip_configuration {
    name                          = "primary"
    subnet_id                     = var.subnet_id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = var.public_ip ? azurerm_public_ip.this[0].id : null
  }
}

resource "azurerm_network_interface_security_group_association" "this" {
  count                     = length(var.network_security_group_ids) > 0 ? 1 : 0
  network_interface_id      = azurerm_network_interface.this.id
  network_security_group_id = var.network_security_group_ids[0]
}

resource "azurerm_linux_virtual_machine" "this" {
  name                            = var.name
  resource_group_name             = var.resource_group_name
  location                        = var.location
  size                            = var.size
  admin_username                  = var.admin_username
  disable_password_authentication = true
  network_interface_ids           = [azurerm_network_interface.this.id]
  tags                            = var.tags

  admin_ssh_key {
    username   = var.admin_username
    public_key = tls_private_key.ssh.public_key_openssh
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
    disk_size_gb         = var.os_disk_gb
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = "server"
    version   = "latest"
  }

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_role_assignment" "storage" {
  count                = length(var.storage_account_ids)
  scope                = var.storage_account_ids[count.index]
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azurerm_linux_virtual_machine.this.identity[0].principal_id
}

resource "azurerm_role_assignment" "secrets" {
  count                = length(var.key_vault_ids)
  scope                = var.key_vault_ids[count.index]
  role_definition_name = "Key Vault Secrets User"
  principal_id         = azurerm_linux_virtual_machine.this.identity[0].principal_id
}

output "vm_id" { value = azurerm_linux_virtual_machine.this.id }
output "private_ip" { value = azurerm_network_interface.this.private_ip_address }
output "public_ip" { value = var.public_ip ? azurerm_public_ip.this[0].ip_address : "" }
output "principal_id" { value = azurerm_linux_virtual_machine.this.identity[0].principal_id }
output "ssh_private_key" {
  value     = tls_private_key.ssh.private_key_openssh
  sensitive = true
}
