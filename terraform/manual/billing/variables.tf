variable "billing_account_id" {
  description = "Billing account the Synthify projects are on (XXXXXX-XXXXXX-XXXXXX). Pass it at apply time; it is not committed."
  type        = string

  validation {
    condition     = can(regex("^[0-9A-F]{6}-[0-9A-F]{6}-[0-9A-F]{6}$", trimspace(var.billing_account_id)))
    error_message = "billing_account_id looks like 012345-6789AB-CDEF01 (see `gcloud billing projects describe <project>`)."
  }
}

variable "quota_project_id" {
  description = "Project the Budget API calls are billed to. It needs billingbudgets.googleapis.com enabled, and the person applying needs serviceusage.services.use on it."
  type        = string
  default     = "synthify-491705"
}

variable "budgets" {
  description = "One budget per environment: the project it watches and its monthly amount in currency_code."
  type = map(object({
    project_id     = string
    monthly_amount = number
  }))
  default = {
    stage = {
      project_id     = "synthify-stage-491705"
      monthly_amount = 1000
    }
    prod = {
      project_id     = "synthify-491705"
      monthly_amount = 1000
    }
  }
}

variable "currency_code" {
  description = "Currency of the billing account."
  type        = string
  default     = "JPY"
}

variable "thresholds" {
  description = "Fractions of the monthly amount, in actual spend, that send an email."
  type        = list(number)
  default     = [0.5, 0.9, 1.0]
}
