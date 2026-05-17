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
