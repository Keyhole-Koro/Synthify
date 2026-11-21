variable "project_id" {
  type = string
}

variable "environment" {
  type = string
}

variable "web_base_url" {
  description = "Public web app origin (https://host), probed by the web uptime check."
  type        = string
}

variable "api_base_url" {
  description = "API base URL (https://host), whose /health?ready=1 the API uptime check probes."
  type        = string
}

variable "readiness_monitor_key" {
  description = "Long-lived key the API accepts for /health?ready=1 (SYNTHIFY_READINESS_MONITOR_KEY). Sent in a masked header. Empty sends no header, and the API answers 401."
  type        = string
  default     = ""
  sensitive   = true
}

variable "alert_email" {
  description = "Address that receives these alerts. Comes from a GitHub environment secret; never commit it. Empty creates the policies without a notification channel (incidents show only in the console)."
  type        = string
  default     = ""
  sensitive   = true
}

variable "deploy_service_account_email" {
  description = "Service account CD deploys with. Its admin changes are left out of the admin-change alert. Empty leaves no one out."
  type        = string
  default     = ""
}

variable "runbook_base_url" {
  description = "Base URL of docs/runbooks, linked from each alert."
  type        = string
}

variable "uptime_check_regions" {
  description = "Locations the uptime checks probe from. At least three; the alerts fire when two or more fail."
  type        = list(string)
  default     = ["ASIA_PACIFIC", "EUROPE", "USA_OREGON"]
}
