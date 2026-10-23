variable "foundation" {
  type = object({

    resource_group_id = string, environment_id = string, domain = string,
    registry          = string, location = string
    }
  )
}

variable "images" {
  type = map(string)
  validation {

    condition     = alltrue([for image in values(var.images) : can(regex("@sha256:[a-f0-9]{64}$", image))])
    error_message = "Supply immutable API and processor image digests."

  }

}

variable "secrets" {
  type      = map(string)
  sensitive = true
}

variable "runtime" {
  type = map(string)
}

variable "publish_frontend" {
  type    = bool
  default = false
}

variable "cloudflare_account_id" {
  type = string
}

variable "cloudflare_zone_id" {
  type = string
}

variable "assets_directory" {
  type = string
}

variable "worker_file" {
  type = string
}

locals {

  api_origin       = "https://syntelle-api.${var.foundation.domain}"
  processor_origin = "https://syntelle-processor.${var.foundation.domain}"
  common = {

    SYNTELLE_DEPLOYMENT_MODE            = "cloud"
    SYNTELLE_LOCAL_OWNER                = "0"
    SYNTELLE_APP_ORIGIN                 = "https://app.syntelle.online"
    SYNTELLE_API_ORIGIN                 = local.api_origin
    SYNTELLE_ACCOUNTS                   = "1"
    SYNTELLE_STREAMING                  = "1"
    SYNTELLE_STREAM_TIME_LIMITS_ENABLED = "true"
    SYNTELLE_RENDER_CAPACITY            = "2"
    PROCESSOR_URL                       = local.processor_origin

  }


}

resource "azapi_resource" "app" {

  for_each                  = toset(["api", "processor"])
  schema_validation_enabled = false
  type                      = "Microsoft.App/containerApps@2026-07-01"
  name                      = "syntelle-${each.key}"
  parent_id                 = var.foundation.resource_group_id
  location                  = var.foundation.location
  body = {
    properties = {

      environmentId = var.foundation.environment_id
      configuration = {

        activeRevisionsMode = "Single"
        ingress = {
          external = true, targetPort = each.key == "api" ? 8080 : 8000, allowInsecure = false, transport = "Http"
        }

        registries = [{
          server = var.foundation.registry, username = split(".", var.foundation.registry)[0], passwordSecretRef = "registry"
          }
        ]
        secrets = concat([{
          name = "registry", value = var.secrets["registry"]
          }
          ], [for key, value in var.secrets : {
            name = lower(replace(key, "_", "-")), value = value
          }
        if key != "registry" && (each.key == "api" || key == "SYNTELLE_PROCESSOR_TOKEN")])

      }

      template = {

        scale = {
          minReplicas = 0, maxReplicas = 1
        }

        containers = [{

          name = each.key, image = var.images[each.key]
          resources = {
            cpu = 1, memory = "2Gi"
          }

          env = concat(
            [for key, value in merge(each.key == "api" ? merge(local.common, var.runtime) : {
              SYNTELLE_DEPLOYMENT_MODE = "cloud"
              }
              ) : {
              name = key, value = value
              }
            ],
            [for key, value in var.secrets : {
              name = key, secretRef = lower(replace(key, "_", "-"))
              }
            if key != "registry" && (each.key == "api" || key == "SYNTELLE_PROCESSOR_TOKEN")]
          )
          probes = [{
            type = "Readiness", httpGet = {
              path                 = "/healthz", port = each.key == "api" ? 8080 : 8000
            }, initialDelaySeconds = 5, periodSeconds = 10
            }
          ]

          }
        ]

      }


    }

  }

  response_export_values = ["properties.configuration.ingress.fqdn"]

}

resource "cloudflare_workers_script" "app" {

  count              = var.publish_frontend ? 1 : 0
  account_id         = var.cloudflare_account_id
  script_name        = "syntelle-app"
  content_file       = var.worker_file
  content_sha256     = filesha256(var.worker_file)
  main_module        = "worker.mjs"
  compatibility_date = "2026-10-22"
  assets = {
    directory = var.assets_directory, config = {
      not_found_handling = "single-page-application", run_worker_first = ["/api/*"]
    }

  }

  bindings = [
    {
      name = "ASSETS", type = "assets"
    },
    {
      name = "API_ORIGIN", type = "plain_text", text = "https://${azapi_resource.app["api"].output.properties.configuration.ingress.fqdn}"
    },
    {
      name = "EDGE_TOKEN", type = "secret_text", text = var.secrets["SYNTELLE_EDGE_TOKEN"]
    }

  ]
  depends_on = [azapi_resource.app]
  lifecycle {

    precondition {

      condition     = !var.publish_frontend || (try(var.runtime["SYNTELLE_MODAL_LAUNCH_URL"], "") != "" && (try(var.runtime["SYNTELLE_STUN_URLS"], "") != "" || try(var.runtime["SYNTELLE_TURN_URLS"], "") != ""))
      error_message = "Publication requires the deployed Modal launcher and at least one configured STUN or TURN endpoint."

    }


  }


}

resource "cloudflare_workers_custom_domain" "app" {

  count      = var.publish_frontend ? 1 : 0
  account_id = var.cloudflare_account_id
  zone_id    = var.cloudflare_zone_id
  hostname   = "app.syntelle.online"
  service    = cloudflare_workers_script.app[0].script_name

}

output "api_origin" {
  value = "https://${azapi_resource.app["api"].output.properties.configuration.ingress.fqdn}"
}

output "processor_origin" {
  value = "https://${azapi_resource.app["processor"].output.properties.configuration.ingress.fqdn}"
}
