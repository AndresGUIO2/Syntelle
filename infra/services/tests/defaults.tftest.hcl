mock_provider "azapi" {
  mock_resource "azapi_resource" {
    defaults = { output = { properties = { configuration = { ingress = { fqdn = "fixture.canadacentral.azurecontainerapps.io" } } } } }
  }
}
mock_provider "cloudflare" {}
run "services" {
  command = plan
  variables {
    foundation            = { resource_group_id = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/fixture", environment_id = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/fixture/providers/Microsoft.App/managedEnvironments/fixture", domain = "fixture.canadacentral.azurecontainerapps.io", registry = "fixture.azurecr.io", location = "canadacentral" }
    images                = { api = "fixture@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", processor = "fixture@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
    secrets               = { registry = "fixture", SYNTELLE_EDGE_TOKEN = "fixture" }
    runtime               = {}
    cloudflare_account_id = "fixture"
    cloudflare_zone_id    = "fixture"
    assets_directory      = "../../apps/web/dist"
    worker_file           = "../../deploy/edge/worker.mjs"
  }
  assert {
    condition     = alltrue([for app in azapi_resource.app : app.body.properties.template.scale.minReplicas == 0 && app.body.properties.template.scale.maxReplicas == 1])
    error_message = "Stateful API must not scale horizontally."
  }
  assert {
    condition     = length(cloudflare_workers_script.app) == 0
    error_message = "Frontend must stay unpublished by default."
  }
}
run "computed_endpoints" {
  command = apply
  variables {
    foundation            = { resource_group_id = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/fixture", environment_id = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/fixture/providers/Microsoft.App/managedEnvironments/fixture", domain = "fixture.canadacentral.azurecontainerapps.io", registry = "fixture.azurecr.io", location = "canadacentral" }
    images                = { api = "fixture@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", processor = "fixture@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
    secrets               = { registry = "fixture", SYNTELLE_EDGE_TOKEN = "fixture" }
    runtime               = {}
    cloudflare_account_id = "fixture"
    cloudflare_zone_id    = "fixture"
    assets_directory      = "../../apps/web/dist"
    worker_file           = "../../deploy/edge/worker.mjs"
  }
  assert {
    condition     = output.api_origin == "https://fixture.canadacentral.azurecontainerapps.io" && output.processor_origin == "https://fixture.canadacentral.azurecontainerapps.io"
    error_message = "Endpoint outputs must come from the actual ARM response."
  }
}

run "publish_with_stun_without_turn" {
  command = plan
  variables {
    foundation            = { resource_group_id = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/fixture", environment_id = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/fixture/providers/Microsoft.App/managedEnvironments/fixture", domain = "fixture.canadacentral.azurecontainerapps.io", registry = "fixture.azurecr.io", location = "canadacentral" }
    images                = { api = "fixture@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", processor = "fixture@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
    secrets               = { registry = "fixture", SYNTELLE_EDGE_TOKEN = "fixture" }
    runtime               = { SYNTELLE_MODAL_LAUNCH_URL = "https://render.fixture.modal.run", SYNTELLE_STUN_URLS = "stun:stun.cloudflare.com:3478", SYNTELLE_TURN_URLS = "" }
    publish_frontend      = true
    cloudflare_account_id = "fixture"
    cloudflare_zone_id    = "fixture"
    assets_directory      = "../../apps/web/dist"
    worker_file           = "../../deploy/edge/worker.mjs"
  }
  assert {
    condition     = length(cloudflare_workers_script.app) == 1 && length(cloudflare_workers_custom_domain.app) == 1
    error_message = "STUN-only configuration must allow the frontend and app domain to publish without TURN."
  }
}
