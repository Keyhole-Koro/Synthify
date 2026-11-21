terraform {
  # 1.7 for the removed{} blocks in services/bootstrap and services/platform.
  # State written by a newer Terraform cannot be read by an older one, so CI,
  # CD and local applies move together (CI and CD pin 1.9.8).
  required_version = ">= 1.7.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    newrelic = {
      source  = "newrelic/newrelic"
      version = "~> 3.0"
    }
  }

  # Backend is configured via partial config:
  #   terraform init -backend-config=../backend/stage.hcl
  #   terraform init -backend-config=../backend/prod.hcl
  backend "gcs" {}
}
