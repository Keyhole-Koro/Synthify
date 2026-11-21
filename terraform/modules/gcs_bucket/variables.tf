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

# 7 days, like the Firestore job documents' TTL: long enough to notice and
# undo a bad overwrite or delete, short enough that a user's deleted file
# does not linger.
variable "noncurrent_version_retention_days" {
  description = "Days a version stays after it stops being the live one (overwritten or deleted) before the lifecycle rule deletes it. Live objects are never deleted by this rule."
  type        = number
  default     = 7

  validation {
    # 0 would not be sent to the API (the provider drops zero values), and the
    # rule would then delete every noncurrent version at once.
    condition     = var.noncurrent_version_retention_days >= 1 && floor(var.noncurrent_version_retention_days) == var.noncurrent_version_retention_days
    error_message = "noncurrent_version_retention_days must be a whole number of days, at least 1."
  }
}

variable "cors_origins" {
  description = "Browser origins allowed to upload directly to this bucket via signed URLs."
  type        = list(string)
  default     = []
}
