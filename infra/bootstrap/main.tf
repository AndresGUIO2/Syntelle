terraform {

  backend "local" {}

  required_providers {
    azurerm = {
      source = "hashicorp/azurerm", version = "= 5.10.0"
    }

  }


}

provider "azurerm" {
  storage_use_azuread             = true
  resource_provider_registrations = "none"
  features {

  }

}

resource "azurerm_resource_provider_registration" "deployment" {
  for_each = toset([
    "Microsoft.App",
    "Microsoft.ContainerRegistry",
    "Microsoft.DBforPostgreSQL",
    "Microsoft.Network",
  ])

  name = each.value

  lifecycle {
    prevent_destroy = true
  }
}

variable "storage_account_name" {
  type = string
}

variable "operator_object_id" {
  type = string
}

resource "azurerm_resource_group" "state" {
  name     = "syntelle-tfstate"
  location = "canadacentral"
}

resource "azurerm_storage_account" "state" {

  name                            = var.storage_account_name
  resource_group_name             = azurerm_resource_group.state.name
  location                        = azurerm_resource_group.state.location
  account_tier                    = "Standard"
  account_replication_type        = "LRS"
  min_tls_version                 = "TLS1_2"
  shared_access_key_enabled       = false
  allow_nested_items_to_be_public = false
  blob_properties {
    versioning_enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }


}

resource "azurerm_storage_container" "state" {

  name                  = "tfstate"
  storage_account_id    = azurerm_storage_account.state.id
  container_access_type = "private"
  depends_on            = [azurerm_role_assignment.operator]

}

resource "azurerm_role_assignment" "operator" {

  scope                = azurerm_storage_account.state.id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = var.operator_object_id

}

output "backend" {
  value = {

    resource_group_name  = azurerm_resource_group.state.name
    storage_account_name = azurerm_storage_account.state.name
    container_name       = azurerm_storage_container.state.name
    use_azuread_auth     = true

  }

}
