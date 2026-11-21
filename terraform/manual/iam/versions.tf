terraform {
  # 1.7 for import blocks with for_each.
  required_version = ">= 1.7.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }

  # Partial config, given at init (see README.md): the environment's state
  # bucket, under its own prefix next to CD's.
  backend "gcs" {}
}
