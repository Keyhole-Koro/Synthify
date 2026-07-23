output "uri" {
  value = module.service.uri
}

output "name" {
  value = module.service.name
}

output "domain" {
  description = "Custom domain mapped to the service, or empty if none."
  value       = var.domain
}

# DNS records Cloud Run expects for the custom domain. Add these to the
# keyhole.work zone. Empty when no domain is mapped.
output "dns_records" {
  value = var.domain == "" ? [] : google_cloud_run_domain_mapping.this[0].status[0].resource_records
}
