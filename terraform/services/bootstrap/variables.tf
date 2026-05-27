variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "environment" {
  type = string
}

variable "uploads_bucket_name" {
  type = string
}

variable "uploads_bucket_cors_origins" {
  description = "Browser origins allowed to upload directly to the uploads bucket via signed URLs."
  type        = list(string)
  default     = []
}

# CI/WIF principal that runs terraform apply. Empty => skip the deployer
# bindings (a human with broad IAM applying locally doesn't need them).
variable "deployer_principal" {
  type    = string
  default = ""
}

# Project-level roles bound to deployer_principal so the deployer can manage
# every resource in the root config. See the root variable for the rationale.
variable "deployer_project_roles" {
  type    = list(string)
  default = []
}

# "signed" => the API signs upload URLs via IAM SignBlob and needs
# serviceAccountTokenCreator on itself. Anything else => skip.
variable "gcs_upload_issuer" {
  type    = string
  default = ""
}
