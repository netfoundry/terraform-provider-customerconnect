resource "customer-connect_location" "example" {
  customer_id = "00000000-0000-0000-0000-000000000001"
  name        = "NYC Office"
  description = "New York City headquarters"
  address     = "1 World Trade Center, New York, NY 10007"
  latitude    = 40.7127
  longitude   = -74.0134
}

# Virtual location hosted on a cloud provider
resource "customer-connect_location" "cloud" {
  customer_id    = "00000000-0000-0000-0000-000000000001"
  name           = "AWS US-East-1"
  description    = "Virtual location in AWS us-east-1"
  virtual        = true
  cloud_provider = "AWS"
  cloud_region   = "us-east-1"
}