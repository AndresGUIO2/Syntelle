mock_provider "azurerm" {
  mock_data "azurerm_client_config" { defaults = { subscription_id = "00000000-0000-0000-0000-000000000001" } }
  mock_data "azurerm_resource_group" {
    defaults = {
      id       = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/rg-syntelle"
      name     = "rg-syntelle"
      location = "eastus"
    }
  }
}
mock_provider "azapi" {}
run "defaults" {
  command = plan
  variables { database_password = "isolated-fixture-password" }
  assert {
    condition     = azurerm_postgresql_flexible_server.database.public_network_access_enabled == false
    error_message = "Database must stay private."
  }
  assert {
    condition     = azurerm_postgresql_flexible_server.database.storage_mb == 32768 && azurerm_postgresql_flexible_server.database.version == "18" && azurerm_postgresql_flexible_server.database.zone == "1"
    error_message = "Unexpected database allocation."
  }
  assert {
    condition     = azapi_resource.express.body.properties.environmentMode == "Express"
    error_message = "Express must not become standard Consumption."
  }
  assert {
    condition     = !contains(keys(azapi_resource.express.body.properties), "appLogsConfiguration")
    error_message = "Omit appLogsConfiguration; Azure rejects the literal none destination."
  }
  assert {
    condition     = azurerm_postgresql_flexible_server.database.sku_name == "B_Standard_B1ms" && azurerm_postgresql_flexible_server.database.backup_retention_days == 7 && length(azurerm_postgresql_flexible_server.database.high_availability) == 0
    error_message = "Keep the approved burstable database and backup defaults."
  }
}
