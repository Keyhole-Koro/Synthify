variable "project_id" {
  type = string
}

variable "name" {
  type = string
}

variable "location" {
  type = string
}

variable "force_destroy" {
  type    = bool
  default = false
}

variable "versioning" {
  description = "Keep non-current object versions so overwrites/deletes are recoverable."
  type        = bool
  default     = true
}

variable "noncurrent_version_retention_days" {
  description = "Days a non-current object version is kept before it is deleted. Only applies with versioning."
  type        = number
  default     = 7
}

variable "cors_origins" {
  description = "Browser origins allowed to upload directly to this bucket via signed URLs."
  type        = list(string)
  default     = []
}
