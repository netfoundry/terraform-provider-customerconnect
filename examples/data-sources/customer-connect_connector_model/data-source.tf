# Look up a Connector Model by its ID
data "customerconnect_connector_model" "example" {
  id = "00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "connector_model_name" {
  value = data.customerconnect_connector_model.example.name
}

output "connector_model_type" {
  value = data.customerconnect_connector_model.example.type
}

output "connector_model_applications" {
  value = data.customerconnect_connector_model.example.applications
}

output "connector_model_counts" {
  value = data.customerconnect_connector_model.example.counts
}
