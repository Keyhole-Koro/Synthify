output "budget_names" {
  description = "Budget resource names, by environment."
  value       = { for env, budget in google_billing_budget.this : env => budget.name }
}
