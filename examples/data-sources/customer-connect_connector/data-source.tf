# Look up a Connector by its ID
data "customerconnect_connector" "example" {
  id = "00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "connector_name" {
  value = data.customerconnect_connector.example.name
}

output "connector_type" {
  value = data.customerconnect_connector.example.type
}

output "connector_enrolled" {
  value = data.customerconnect_connector.example.enrolled
}

output "connector_online" {
  value = data.customerconnect_connector.example.online
}
