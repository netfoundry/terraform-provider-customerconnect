terraform {
  required_providers {
    customer-connect = {
      source  = "netfoundry/customer-connect"
      version = "~> 0.1"
    }
  }
}

provider "customer-connect" {
  # Cognito OAuth2 client credentials for the NetFoundry API.
  # These can also be supplied via NF_CLIENT_ID and NF_CLIENT_SECRET environment variables.
  client_id     = var.nf_client_id
  client_secret = var.nf_client_secret

  # Deploy environment: "sandbox", "staging", or "production" (default).
  # Can also be set via NF_ENVIRONMENT.
  environment = "production"
}

variable "nf_client_id" {
  description = "NetFoundry API client ID."
  type        = string
  sensitive   = false
}

variable "nf_client_secret" {
  description = "NetFoundry API client secret."
  type        = string
  sensitive   = true
}
