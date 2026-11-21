terraform {
  required_version = ">= 1.6.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }

  # Partial config, given at init (see README.md). The state sits next to
  # prod's, under its own prefix.
  backend "gcs" {}
}
