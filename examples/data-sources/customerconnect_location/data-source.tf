# Look up a Location by its ID
data "customerconnect_location" "example" {
  id = "00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "location_name" {
  value = data.customerconnect_location.example.name
}

output "location_enabled" {
  value = data.customerconnect_location.example.enabled
}
