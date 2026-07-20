# Look up an AccessPolicy by its ID
data "customerconnect_access_policy" "example" {
  id = "00000000-0000-0000-0000-000000000001"
}

# Reference computed attributes from the data source
output "access_policy_name" {
  value = data.customerconnect_access_policy.example.name
}

output "access_policy_enabled" {
  value = data.customerconnect_access_policy.example.enabled
}

output "access_policy_ziti_name" {
  value = data.customerconnect_access_policy.example.ziti_name
}

output "access_policy_sources" {
  value = data.customerconnect_access_policy.example.sources
}

output "access_policy_destinations" {
  value = data.customerconnect_access_policy.example.destinations
}
