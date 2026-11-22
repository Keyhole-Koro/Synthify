output "workload_identity_provider" {
  description = "Value of the GitHub environment secret GCP_WIF_PROVIDER."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "deploy_service_account_email" {
  description = "Value of the GitHub environment secret GCP_WIF_SA_EMAIL, and of deployer_principal (with serviceAccount:) in terraform/tfvars/<env>.tfvars."
  value       = google_service_account.deploy.email
}

output "runtime_service_account_emails" {
  value = { for key, account in google_service_account.runtime : key => account.email }
}
