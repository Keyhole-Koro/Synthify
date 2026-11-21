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

# A worker instance runs one job at a time, so the worker can take only as
# many dispatches as it has instances. Dispatches past that are refused with
# 429 and each refusal spends one of the task's attempts; tasks held back by
# this limit wait in the queue instead.
variable "pipeline_max_concurrent_dispatches" {
  description = "Most tasks the pipeline queue dispatches at once. Keep it equal to the worker's max instance count."
  type        = number
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
    "aiplatform.googleapis.com"
  ]
}
