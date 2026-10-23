terraform {

  required_version = ">= 1.11, < 2"
  required_providers {

    azapi = {
      source = "Azure/azapi", version = "= 2.13.0"
    }

    cloudflare = {
      source = "cloudflare/cloudflare", version = "= 5.28.1"
    }


  }

  backend "azurerm" {

  }


}

provider "azapi" {

}

provider "cloudflare" {

}
