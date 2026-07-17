# Look up a Customer by its ID
data "customer-connect_customer" "example" {
  id = "00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "customer_name" {
  value = data.customer-connect_customer.example.name
}

output "customer_enabled" {
  value = data.customer-connect_customer.example.enabled
}
