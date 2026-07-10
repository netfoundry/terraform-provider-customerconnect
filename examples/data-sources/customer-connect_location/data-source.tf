# Look up a Location by its ID
data "customer-connect_location" "example" {
  id = "loc-00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "location_name" {
  value = data.customer-connect_location.example.name
}

output "location_enabled" {
  value = data.customer-connect_location.example.enabled
}
