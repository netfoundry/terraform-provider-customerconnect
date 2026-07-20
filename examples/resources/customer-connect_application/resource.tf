resource "customerconnect_location" "example" {
  customer_id = "00000000-0000-0000-0000-000000000001"
  name        = "NYC Office"
}

resource "customerconnect_connector" "example" {
  location_id = customerconnect_location.example.id
  name        = "nyc-gateway-01"
  type        = "GATEWAY"
}

# Application with a single service address mapping
resource "customerconnect_application" "web" {
  connector_id = customerconnect_connector.example.id
  name         = "internal-web"
  description  = "Internal web application"
  type         = "HTTP"
  protocol     = "TCP"

  addresses = [
    {
      listen_address = ["web.internal"]
      listen_port    = ["443"]
      target_address = "10.0.1.10"
      target_port    = 443
    }
  ]
}

# Application with multiple address mappings and forwarding enabled
resource "customerconnect_application" "multi" {
  connector_id = customerconnect_connector.example.id
  name         = "multi-port-app"
  type         = "custom"
  protocol     = "TCP_UDP"

  addresses = [
    {
      listen_address = ["10.1.0.1"]
      listen_port    = ["7000"]
      target_address = "10.1.0.2"
      target_port    = 7000
    },
    {
      listen_address    = ["10.1.0.3"]
      listen_port       = ["7001-7010"]
      forward_address   = true
      allowed_addresses = ["10.1.0.0/24"]
      forward_port      = true
      allowed_ports     = ["7001-7010"]
    }
  ]
}
