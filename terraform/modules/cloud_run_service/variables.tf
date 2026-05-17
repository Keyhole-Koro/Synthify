variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "name" {
  type = string
}

variable "image" {
  type = string
}

variable "service_account_email" {
  type = string
}

variable "env_vars" {
  type    = map(string)
  default = {}
}

variable "secret_env_vars" {
  type = list(object({
    name    = string
    secret  = string
    version = optional(string, "latest")
  }))
  default = []
}

# GCS bucket mounted into the container via Cloud Run's gcsfuse integration.
# When set, the bucket is mounted read-only at mount_path so the app can read
# uploaded objects as local files (no signed URLs / HTTP fetch). Null = no mount.
variable "gcs_volume" {
  type = object({
    bucket     = string
    mount_path = string
  })
  default = null
}

variable "min_instance_count" {
  type    = number
  default = 0
}

variable "max_instance_count" {
  type    = number
  default = 2
}

variable "ingress" {
  type    = string
  default = "INGRESS_TRAFFIC_ALL"
}

variable "allow_unauthenticated" {
  type    = bool
  default = false
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "cpu" {
  type    = string
  default = "1"
}

variable "memory" {
  type    = string
  default = "512Mi"
}

variable "timeout" {
  type    = string
  default = "300s"
}

# Cloud Run v2 defaults deletion_protection to true. Set false in non-prod so
# Terraform can replace a tainted/failed service without manual intervention.
variable "deletion_protection" {
  type    = bool
  default = true
}
