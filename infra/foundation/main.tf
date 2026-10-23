variable "resource_group_name" {
  type    = string
  default = "rg-syntelle"
}

variable "location" {
  type    = string
  default = "canadacentral"
}

variable "database_password" {
  type      = string
  sensitive = true
}

data "azurerm_client_config" "current" {

}

locals {
  suffix = substr(sha256(data.azurerm_client_config.current.subscription_id), 0, 8)
}

data "azurerm_resource_group" "main" {
  name = var.resource_group_name
}

resource "azurerm_virtual_network" "main" {
  name                = "syntelle"
  location            = var.location
  resource_group_name = data.azurerm_resource_group.main.name
  address_space       = ["10.44.0.0/16"]
}

resource "azurerm_subnet" "apps" {
  name                 = "express"
  resource_group_name  = data.azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.44.0.0/23"]
  delegation {
    name = "express"
    service_delegation {
      name    = "Microsoft.App/environments"
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }

  }

}

resource "azurerm_subnet" "database" {
  name                 = "postgres"
  resource_group_name  = data.azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.44.2.0/24"]
  delegation {
    name = "postgres"
    service_delegation {
      name    = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }

  }

}

resource "azurerm_private_dns_zone" "database" {
  name                = "syntelle.private.postgres.database.azure.com"
  resource_group_name = data.azurerm_resource_group.main.name
}

resource "azurerm_private_dns_zone_virtual_network_link" "database" {
  name                = "syntelle"
  private_dns_zone_id = azurerm_private_dns_zone.database.id
  virtual_network_id  = azurerm_virtual_network.main.id
}

resource "azurerm_postgresql_flexible_server" "database" {

  name                          = "syntelle-${local.suffix}"
  resource_group_name           = data.azurerm_resource_group.main.name
  location                      = var.location
  version                       = "18"
  administrator_login           = "syntelle"
  administrator_password        = var.database_password
  sku_name                      = "B_Standard_B1ms"
  storage_mb                    = 32768
  backup_retention_days         = 7
  delegated_subnet_id           = azurerm_subnet.database.id
  private_dns_zone_id           = azurerm_private_dns_zone.database.id
  public_network_access_enabled = false
  depends_on                    = [azurerm_private_dns_zone_virtual_network_link.database]
  lifecycle {
    prevent_destroy = true
  }


}

resource "azurerm_postgresql_flexible_server_database" "app" {
  name      = "syntelle"
  server_id = azurerm_postgresql_flexible_server.database.id
  collation = "en_US.utf8"
  charset   = "UTF8"
  lifecycle {
    prevent_destroy = true
  }

}

resource "azurerm_container_registry" "main" {
  name                = "syntelle${local.suffix}"
  resource_group_name = data.azurerm_resource_group.main.name
  location            = var.location
  sku                 = "Basic"
  admin_enabled       = true
}

resource "azapi_resource" "express" {

  schema_validation_enabled = false
  type                      = "Microsoft.App/managedEnvironments@2026-07-01"
  name                      = "syntelle-express"
  parent_id                 = data.azurerm_resource_group.main.id
  location                  = var.location
  body = {
    properties = {

      environmentMode = "Express"
      appLogsConfiguration = {
        destination = "none"
      }

      vnetConfiguration = {
        infrastructureSubnetId = azurerm_subnet.apps.id
      }


    }

  }

  response_export_values = ["properties.defaultDomain"]

}

output "foundation" {
  value = {

    resource_group_id = data.azurerm_resource_group.main.id
    environment_id    = azapi_resource.express.id
    domain            = azapi_resource.express.output.properties.defaultDomain
    registry          = azurerm_container_registry.main.login_server
    registry_name     = azurerm_container_registry.main.name
    database_fqdn     = azurerm_postgresql_flexible_server.database.fqdn
    location          = var.location

  }

}

output "registry_password" {
  value     = azurerm_container_registry.main.admin_password
  sensitive = true
}
