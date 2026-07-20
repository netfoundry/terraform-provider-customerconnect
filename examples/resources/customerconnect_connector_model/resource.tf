# Connector model with a single application and address mapping
resource "customerconnect_connector_model" "web_gateway" {
  provider_id = "00000000-0000-0000-0000-000000000002"
  name        = "web-gateway-model"
  description = "Standard gateway model exposing an internal web application"
  type        = "GATEWAY"

  applications = [
    {
      name     = "internal-web"
      type     = "HTTP"
      protocol = "TCP"

      addresses = [
        {
          listen_address = ["web.internal"]
          listen_port    = ["443"]
          target_address = "10.0.1.10"
          target_port    = "443"
        }
      ]
    }
  ]
}

# Connector model with multiple applications, including one left for
# connectors to override via required fields
resource "customerconnect_connector_model" "multi" {
  provider_id = "00000000-0000-0000-0000-000000000002"
  name        = "multi-app-model"
  type        = "DEVICE"

  applications = [
    {
      name     = "ssh-access"
      type     = "SSH"
      protocol = "TCP"

      addresses = [
        {
          listen_address = ["{{location.name}}-ssh"]
          listen_port    = ["22"]
          target_address = "10.1.0.2"
          target_port    = "22"
        }
      ]
    },
    {
      name     = "printer"
      type     = "custom"
      protocol = "TCP_UDP"
      # addresses omitted — connectors attaching this model must supply them via override
    }
  ]
}

# SDK-embedded connector model (no type/protocol/addresses on applications)
resource "customerconnect_connector_model" "sdk" {
  provider_id = "00000000-0000-0000-0000-000000000002"
  name        = "sdk-embedded-model"
  type        = "SDK_EMBEDDED"

  applications = [
    {
      name = "embedded-app"
    }
  ]
}
