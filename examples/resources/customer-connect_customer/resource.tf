resource "customer-connect_customer" "example" {
  provider_id = "00000000-0000-0000-0000-000000000001"
  name        = "NF Corp"
  description = "NF Corp customer account"
}

# Customer created disabled
resource "customer-connect_customer" "disabled" {
  provider_id = "00000000-0000-0000-0000-000000000001"
  name        = "Suspended Customer"
  enabled     = false
}
