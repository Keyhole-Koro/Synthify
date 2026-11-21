# Monthly budgets for the Synthify projects, applied by hand (README.md).
#
# A budget belongs to the billing account, not to a project, so creating one
# needs a role on the billing account (Billing Account Administrator or Costs
# Manager). The deploy service account has no role there, and giving it one
# would let it read and change the budgets of every project on the account.
# CD therefore never applies this stack.

provider "google" {
  # With user credentials the Budget API needs a quota project that has
  # billingbudgets.googleapis.com enabled (services/platform enables it).
  billing_project       = var.quota_project_id
  user_project_override = true
}

data "google_project" "this" {
  for_each   = var.budgets
  project_id = each.value.project_id
}

resource "google_billing_budget" "this" {
  for_each = var.budgets

  billing_account = trimspace(var.billing_account_id)
  display_name    = "synthify-${each.key}-budget"

  budget_filter {
    # The API stores projects by number; an id here would show as a change on
    # every plan.
    projects               = ["projects/${data.google_project.this[each.key].number}"]
    credit_types_treatment = "INCLUDE_ALL_CREDITS"
  }

  amount {
    specified_amount {
      # Must be the billing account's currency, or the API rejects the budget.
      currency_code = var.currency_code
      units         = tostring(each.value.monthly_amount)
    }
  }

  dynamic "threshold_rules" {
    for_each = var.thresholds
    content {
      threshold_percent = threshold_rules.value
    }
  }

  # Also when the month is forecast to go over, so a sudden rise is seen
  # before the money is spent.
  threshold_rules {
    threshold_percent = 1.0
    spend_basis       = "FORECASTED_SPEND"
  }

  # No all_updates_rule: the emails go to the billing account's administrators
  # and users, which is the default.
}
