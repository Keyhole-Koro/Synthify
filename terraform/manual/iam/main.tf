# Service accounts, IAM and the GitHub Workload Identity provider for one
# environment. Applied by hand by a project owner (README.md), never by CD.
#
# CD deploys as the github-deploy service account, which any workflow allowed
# by the Workload Identity provider can become. It used to hold
# projectIamAdmin, serviceAccountAdmin and serviceAccountTokenCreator on the
# whole project, so it could grant itself any role: whoever could run a
# workflow could take the project. Those three roles are removed below, and
# everything that needed them lives here instead: creating service accounts,
# project IAM, and IAM on service accounts.

provider "google" {
  project = var.project_id
}

locals {
  # Runtime service accounts that CD's Cloud Run services and jobs run as.
  # services/platform computes the same emails.
  runtime_service_accounts = {
    api            = "API"
    worker         = "Worker"
    eval           = "Eval"
    eval_scheduler = "Eval Scheduler"
    monitor        = "Monitor"
  }

  runtime_project_roles = {
    api = [
      # Job status documents in Firestore (workspaces/{ws}/jobs/{job}).
      "roles/datastore.user",
      # CreateTask on the pipeline queue, to dispatch jobs to the worker.
      "roles/cloudtasks.enqueuer",
      # Firebase Auth: the custom role api_firebase_auth_users, below.
    ]
    worker = [
      "roles/datastore.user",
      # Gemini on Vertex AI.
      "roles/aiplatform.user",
    ]
    eval = [
      "roles/aiplatform.user",
    ]
  }

  runtime_project_bindings = merge([
    for account, roles in local.runtime_project_roles : {
      for role in roles : "${account} ${role}" => { account = account, role = role }
    }
  ]...)

  # Never again on the deploy account: each lets it grant itself more.
  # projectIamAdmin sets project IAM; serviceAccountAdmin sets IAM on (and
  # creates) service accounts; serviceAccountTokenCreator on the project lets
  # it act as every service account in it.
  deploy_removed_roles = [
    "roles/resourcemanager.projectIamAdmin",
    "roles/iam.serviceAccountAdmin",
    "roles/iam.serviceAccountTokenCreator",
  ]

  # The curated Firebase Auth roles CD granted the API before this stack:
  # viewer (#55) and admin (#75). CD's removed blocks leave them in place; the
  # custom role api_firebase_auth_users replaces both.
  api_removed_roles = [
    "roles/firebaseauth.admin",
    "roles/firebaseauth.viewer",
  ]

  deploy_member = google_service_account.deploy.member
}

# What this stack needs on a new project: service accounts, and the token
# exchange Workload Identity uses. CD's platform module also enables
# iam.googleapis.com; enabling twice is harmless, and neither disables.
resource "google_project_service" "identity" {
  for_each = toset([
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "sts.googleapis.com",
    "cloudresourcemanager.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# --------------------------------------------------------------------
# Service accounts
# --------------------------------------------------------------------

# The accounts already exist in stage and prod (created by hand, or by CD
# before this stack). create_ignore_already_exists adopts an existing account
# instead of failing.
resource "google_service_account" "runtime" {
  for_each = local.runtime_service_accounts

  project                      = var.project_id
  account_id                   = "synthify-${replace(each.key, "_", "-")}-${var.environment}"
  display_name                 = "Synthify ${each.value} (${var.environment})"
  create_ignore_already_exists = true

  depends_on = [google_project_service.identity]

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_service_account" "deploy" {
  project                      = var.project_id
  account_id                   = var.deploy_account_id
  display_name                 = "GitHub Deploy"
  create_ignore_already_exists = true

  depends_on = [google_project_service.identity]

  lifecycle {
    prevent_destroy = true
  }
}

# --------------------------------------------------------------------
# GitHub Actions -> deploy account (Workload Identity Federation)
# --------------------------------------------------------------------

# The pool and provider exist in stage and prod (created by hand). Import
# them; a new project sets adopt_existing_workload_identity = false.
import {
  for_each = var.adopt_existing_workload_identity ? toset(["pool"]) : toset([])
  to       = google_iam_workload_identity_pool.github
  id       = "projects/${var.project_id}/locations/global/workloadIdentityPools/${var.workload_identity_pool_id}"
}

import {
  for_each = var.adopt_existing_workload_identity ? toset(["provider"]) : toset([])
  to       = google_iam_workload_identity_pool_provider.github
  id       = "projects/${var.project_id}/locations/global/workloadIdentityPools/${var.workload_identity_pool_id}/providers/${var.workload_identity_provider_id}"
}

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = var.workload_identity_pool_id
  display_name              = "GitHub Actions"

  depends_on = [google_project_service.identity]

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = var.workload_identity_provider_id
  display_name                       = "GitHub"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }

  # Only this environment's deploy jobs: the job must run in the GitHub
  # environment of the same name (deploy-backend.yml and deploy-frontend.yml
  # set `environment:`), from the branch of the same name. The condition used
  # to check the repository only, so any workflow in it could become the
  # deploy account from any branch: a workflow_dispatch from a feature branch,
  # a pull request from a branch of this repository, or a new workflow file.
  attribute_condition = join(" && ", [
    "assertion.repository == '${var.github_repository}'",
    "assertion.environment == '${var.environment}'",
    "assertion.ref == 'refs/heads/${var.environment}'",
  ])

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  lifecycle {
    prevent_destroy = true
  }
}

data "google_iam_policy" "deploy" {
  binding {
    role    = "roles/iam.workloadIdentityUser"
    members = ["principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repository}"]
  }
}

# The whole IAM policy of the deploy account: only GitHub Actions, through the
# provider above, may act as it. Authoritative, so a grant added to the
# account by hand (a role on itself, a key creator) is removed on apply.
resource "google_service_account_iam_policy" "deploy" {
  service_account_id = google_service_account.deploy.name
  policy_data        = data.google_iam_policy.deploy.policy_data
}

# --------------------------------------------------------------------
# What the deploy account may do
# --------------------------------------------------------------------

resource "google_project_iam_member" "deploy" {
  for_each = toset(var.deploy_project_roles)

  project = var.project_id
  role    = each.value
  member  = local.deploy_member
}

resource "google_project_iam_member_remove" "deploy" {
  for_each = toset(local.deploy_removed_roles)

  project = var.project_id
  role    = each.value
  member  = local.deploy_member

  lifecycle {
    precondition {
      condition     = !contains(var.deploy_project_roles, each.value)
      error_message = "${each.value} must not be in deploy_project_roles: it lets the deploy account grant itself any role."
    }
  }
}

# Attaching a service account to a Cloud Run service or job, or to a Cloud
# Scheduler job's OAuth token, needs actAs on that account.
resource "google_service_account_iam_member" "deploy_acts_as" {
  for_each = google_service_account.runtime

  service_account_id = each.value.name
  role               = "roles/iam.serviceAccountUser"
  member             = local.deploy_member
}

# --------------------------------------------------------------------
# What the runtime accounts may do
# --------------------------------------------------------------------

resource "google_project_iam_member" "runtime" {
  for_each = local.runtime_project_bindings

  project = var.project_id
  role    = each.value.role
  member  = google_service_account.runtime[each.value.account].member
}

# The API on itself:
# - serviceAccountUser: CreateTask with an OIDC token for the API account
#   needs actAs on it (Cloud Tasks pushes jobs to the worker as the API).
# - serviceAccountTokenCreator: Cloud Tasks mints that OIDC token, and the
#   API signs upload URLs with IAM SignBlob (no private key).
resource "google_service_account_iam_member" "api_self" {
  for_each = toset([
    "roles/iam.serviceAccountUser",
    "roles/iam.serviceAccountTokenCreator",
  ])

  service_account_id = google_service_account.runtime["api"].name
  role               = each.value
  member             = google_service_account.runtime["api"].member
}

# Firebase Authentication users, for the API: the invitation check reads the
# invitee's account (GetUser, #55) and account deletion deletes the user
# (DeleteUser, #75). No curated role fits. viewer cannot delete; admin, the
# only one that can, can also change the sign-in settings, read the password
# hash settings, and create, edit or email any user.
resource "google_project_iam_custom_role" "api_firebase_auth_users" {
  project     = var.project_id
  role_id     = "synthifyApiFirebaseAuthUsers"
  title       = "Synthify API: Firebase Auth users"
  description = "Read and delete Firebase Authentication users, for the API's invitation check and account deletion."
  permissions = [
    "firebaseauth.users.delete",
    "firebaseauth.users.get",
  ]

  depends_on = [google_project_service.identity]
}

resource "google_project_iam_member" "api_firebase_auth_users" {
  project = var.project_id
  role    = google_project_iam_custom_role.api_firebase_auth_users.name
  member  = google_service_account.runtime["api"].member
}

# The curated roles come off only after the custom role is granted, so the API
# is not left without the permissions in between (IAM can still take a few
# minutes to apply either change). Nothing happens where they are not granted.
resource "google_project_iam_member_remove" "api" {
  for_each = toset(local.api_removed_roles)

  project = var.project_id
  role    = each.value
  member  = google_service_account.runtime["api"].member

  depends_on = [google_project_iam_member.api_firebase_auth_users]

  lifecycle {
    precondition {
      condition     = !contains(local.runtime_project_roles.api, each.value)
      error_message = "${each.value} must not be in runtime_project_roles.api: the custom role api_firebase_auth_users replaces it."
    }
  }
}
