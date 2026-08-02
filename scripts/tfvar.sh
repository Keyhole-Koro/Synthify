#!/usr/bin/env bash
#
# Print a single value from terraform/tfvars/<env>.tfvars.
#
# tfvars is where non-secret stage/prod configuration lives, and it is reviewable
# in the repository. Deploy workflows used to restate some of the same facts as
# GitHub Environment Variables — project id and region existed in both — which
# meant the effective value could not be known by reading the repository, and
# the two could drift the way compose and web.yml did.
#
# Usage: scripts/tfvar.sh stage project_id
set -euo pipefail

env_name="${1:?usage: tfvar.sh <env> <key>}"
key="${2:?usage: tfvar.sh <env> <key>}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
file="${repo_root}/terraform/tfvars/${env_name}.tfvars"

if [ ! -f "$file" ]; then
  echo "no tfvars for environment '${env_name}' (${file})" >&2
  exit 1
fi

# Only plain `key = "value"` assignments. Anything else (a list, an
# interpolation) is out of scope on purpose: this exists to read the handful of
# scalars the deploy pipeline needs before Terraform runs, not to parse HCL.
value="$(sed -nE "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*\"([^\"]*)\".*/\1/p" "$file" | head -n 1)"

if [ -z "$value" ]; then
  echo "key '${key}' not found in ${file}" >&2
  exit 1
fi

printf '%s\n' "$value"
