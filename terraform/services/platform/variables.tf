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

variable "required_services" {
  type = set(string)
  default = [
    "run.googleapis.com",
    "cloudtasks.googleapis.com",
    "cloudscheduler.googleapis.com",
    "secretmanager.googleapis.com",
    "storage.googleapis.com",
    "iam.googleapis.com",
    "aiplatform.googleapis.com",
    # The budgets in terraform/manual/billing are applied by hand with user
    # credentials, and the Budget API then needs a quota project that has
    # this API on. Either environment's project can be that project.
    "billingbudgets.googleapis.com",
  ]
}
