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

variable "sensitive_env_vars" {
  type      = map(string)
  default   = {}
  sensitive = true
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
# When set, the bucket is mounted at mount_path so the app can read uploaded
# objects as local files (no signed URLs / HTTP fetch). read_only defaults to
# true; the worker sets it false so it can write job checkpoints under the
# mount. Null = no mount.
variable "gcs_volume" {
  type = object({
    bucket     = string
    mount_path = string
    read_only  = optional(bool, true)
  })
  default = null
}

variable "min_instance_count" {
  type    = number
  default = 0
}

# CPU allocation / billing mode. true = CPU is only allocated while a request is
# in flight (request-based billing); false = CPU is always allocated for the
# whole instance lifetime (instance-based billing).
#
# This MUST stay explicit. google_cloud_run_v2_service only defaults cpu_idle to
# true when the `resources` block is absent entirely; because this module always
# sets resources.limits, leaving cpu_idle unset makes the provider send false and
# the service is billed for every second an instance is alive, even idle ones —
# min_instance_count=0 saves nothing once anything (a health probe, a scheduler)
# keeps an instance warm. See hashicorp/terraform-provider-google#17246.
#
# Consequence of true: goroutines/timers that run outside a request are throttled
# and must not be relied on. Periodic work belongs in Cloud Scheduler (see the
# maintenance sweep in services/api) or a Cloud Run Job.
variable "cpu_idle" {
  type    = bool
  default = true
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
