# Look up a Connector by its ID
data "customer_connect_connector" "example" {
  id = "00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "connector_name" {
  value = data.customer_connect_connector.example.name
}

output "connector_type" {
  value = data.customer_connect_connector.example.type
}

output "connector_enrolled" {
  value = data.customer_connect_connector.example.enrolled
}

output "connector_online" {
  value = data.customer_connect_connector.example.online
}
