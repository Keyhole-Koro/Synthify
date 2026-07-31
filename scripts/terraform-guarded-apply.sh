#!/usr/bin/env bash
#
# Plan, refuse to continue if the plan destroys anything, then apply the exact
# plan that was inspected.
#
# Why this exists: the deploy pipeline applies without -target in Pass 2, so a
# deploy from a branch whose Terraform config is older than what was last
# applied to that environment silently proposes destroying every resource the
# branch does not know about. The stage/prod branches are cherry-pick deploy
# branches that drift behind main by design, and an environment can also be
# applied out-of-band via workflow_dispatch from another ref — both leave the
# state ahead of the config. -auto-approve means nothing stops it.
#
# Usage:
#   scripts/terraform-guarded-apply.sh -var-file=../tfvars/stage.tfvars -var=...
#
# Pass the plan arguments only. Do NOT pass -auto-approve: the apply consumes a
# saved plan file, which never prompts and rejects re-specified variables.
#
# Set ALLOW_DESTROY=true to proceed anyway, for the case where the destroy is
# the intended change (a resource genuinely removed from config).
set -euo pipefail

plan_file="$(mktemp -t tfplan.XXXXXX)"
plan_json="${plan_file}.json"
trap 'rm -f "$plan_file" "$plan_json"' EXIT

terraform plan -out="$plan_file" "$@"
terraform show -json "$plan_file" >"$plan_json"

# A pure destroy is exactly ["delete"]. A replace is ["delete","create"] or
# ["create","delete"] and is deliberately allowed: that is how Terraform
# updates an immutable field, it happens during ordinary deploys, and blocking
# it would block them.
mapfile -t destroys < <(
  jq -r '
    .resource_changes[]?
    | select(.change.actions == ["delete"])
    | .address
  ' "$plan_json"
)

if [ ${#destroys[@]} -gt 0 ]; then
  if [ "${ALLOW_DESTROY:-}" = "true" ]; then
    echo "::warning::plan destroys ${#destroys[@]} resource(s); continuing because ALLOW_DESTROY=true"
    printf '  %s\n' "${destroys[@]}"
  else
    echo "::error::Refusing to apply: the plan destroys ${#destroys[@]} resource(s)"
    printf '::error::  %s\n' "${destroys[@]}"
    cat >&2 <<'EOF'

This usually means the checked-out config is older than the state — the branch
being deployed does not contain something that was already applied to this
environment. Sync the deploy branch with the config that produced the state
rather than letting the apply remove the resources.

If the destroy IS the intended change, re-run with ALLOW_DESTROY=true.
EOF
    exit 1
  fi
fi

terraform apply "$plan_file"
