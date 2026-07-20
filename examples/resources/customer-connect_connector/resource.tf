resource "customerconnect_location" "example" {
  customer_id = "00000000-0000-0000-0000-000000000001"
  name        = "NYC Office"
}

# GATEWAY connector attached to a location
resource "customerconnect_connector" "gateway" {
  location_id = customerconnect_location.example.id
  name        = "nyc-gateway-01"
  description = "Primary gateway connector for NYC Office"
  type        = "GATEWAY"
}

# DEVICE connector attached to a location
resource "customerconnect_connector" "device" {
  location_id = customerconnect_location.example.id
  name        = "nyc-device-01"
  description = "Device connector for NYC Office"
  type        = "DEVICE"
}

# Connector created from a connector model (type is derived from the model)
resource "customerconnect_connector" "from_model" {
  location_id        = customerconnect_location.example.id
  name               = "nyc-model-connector-01"
  connector_model_id = "00000000-0000-0000-0000-000000000002"
}
