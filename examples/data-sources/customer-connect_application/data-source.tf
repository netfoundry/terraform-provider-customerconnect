# Look up an Application by its connector_id and ID
data "customerconnect_application" "example" {
  connector_id = "00000000-0000-0000-0000-000000000010"
  id           = "00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "application_name" {
  value = data.customerconnect_application.example.name
}

output "application_ziti_id" {
  value = data.customerconnect_application.example.ziti_id
}

output "application_addresses" {
  value = data.customerconnect_application.example.addresses
}
