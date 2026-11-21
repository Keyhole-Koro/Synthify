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
