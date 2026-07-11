terraform {
  required_version = ">= 1.6.0"

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
