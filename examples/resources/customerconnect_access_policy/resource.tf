resource "customerconnect_location" "src" {
  customer_id = "00000000-0000-0000-0000-000000000001"
  name        = "NYC Office"
}

resource "customerconnect_location" "dst" {
  customer_id = "00000000-0000-0000-0000-000000000001"
  name        = "AWS US-East-1"
  virtual     = true
}

# Access policy linking a source location to a destination location
resource "customerconnect_access_policy" "example" {
  provider_id = "00000000-0000-0000-0000-000000000002"
  name        = "nyc-to-aws"
  description = "Allow NYC Office to reach AWS US-East-1"

  sources = [
    {
      location_id = customerconnect_location.src.id
    }
  ]

  destinations = [
    {
      location_id = customerconnect_location.dst.id
    }
  ]
}

# Access policy with multiple sources and destinations using connectors
resource "customerconnect_access_policy" "multi" {
  provider_id = "00000000-0000-0000-0000-000000000002"
  name        = "multi-endpoint-policy"

  sources = [
    {
      connector_id = "00000000-0000-0000-0000-000000000010"
    },
    {
      location_id = customerconnect_location.src.id
    }
  ]

  destinations = [
    {
      connector_id = "00000000-0000-0000-0000-000000000020"
    },
    {
      connector_model_id = "00000000-0000-0000-0000-000000000030"
    }
  ]
}
