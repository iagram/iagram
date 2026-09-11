variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "subnet_id" { type = string }
variable "resource_group_name" { type = string }
variable "location" { type = string }
variable "node_size" {
  type    = string
  default = "Standard_D2s_v5"
}
variable "node_count" {
  type    = number
  default = 2
}
variable "sku_tier" {
  type    = string
  default = "Free"
}
variable "kubernetes_version" {
  type    = string
  default = ""
}
variable "container_registry_ids" {
  type    = list(string)
  default = []
}

resource "azurerm_kubernetes_cluster" "this" {
  name                = var.name
  resource_group_name = var.resource_group_name
  location            = var.location
  dns_prefix          = replace(lower(var.name), "/[^a-z0-9-]/", "-")
  sku_tier            = var.sku_tier
  kubernetes_version  = var.kubernetes_version != "" ? var.kubernetes_version : null
  tags                = var.tags

  default_node_pool {
    name           = "system"
    vm_size        = var.node_size
    node_count     = var.node_count
    vnet_subnet_id = var.subnet_id
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin = "azure"
    service_cidr   = "10.250.0.0/16"
    dns_service_ip = "10.250.0.10"
  }
}

resource "azurerm_role_assignment" "acr_pull" {
  count                = length(var.container_registry_ids)
  scope                = var.container_registry_ids[count.index]
  role_definition_name = "AcrPull"
  principal_id         = azurerm_kubernetes_cluster.this.kubelet_identity[0].object_id
}

output "cluster_name" { value = azurerm_kubernetes_cluster.this.name }
output "cluster_id" { value = azurerm_kubernetes_cluster.this.id }
output "kubelet_identity_object_id" { value = azurerm_kubernetes_cluster.this.kubelet_identity[0].object_id }
