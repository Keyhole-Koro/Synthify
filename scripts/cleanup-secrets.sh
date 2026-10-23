#!/usr/bin/env bash
# Usage: ./scripts/cleanup-secrets.sh <project-id>
# This script keeps the latest version of every secret in the project and destroys all older versions.

set -euo pipefail

PROJECT_ID="${1:-}"

if [[ -z "$PROJECT_ID" ]]; then
  echo "Usage: $0 <project-id>"
  exit 1
fi

echo "Scanning for secrets in project: $PROJECT_ID..."
SECRETS=$(gcloud secrets list --project="$PROJECT_ID" --format="value(name)")

for secret in $SECRETS; do
  echo "Checking secret: $secret"
  
  # Get all enabled/disabled versions sorted by version number descending (latest first)
  ALL_VERSIONS=$(gcloud secrets versions list "$secret" --project="$PROJECT_ID" \
    --filter="state:enabled OR state:disabled" \
    --sort-by="~name" \
    --format="value(name)")
  
  # Count how many active versions exist
  VERSION_COUNT=$(echo "$ALL_VERSIONS" | grep -c . || true)
  
  if [[ "$VERSION_COUNT" -le 1 ]]; then
    echo "  -> Only $VERSION_COUNT active version. Skipping."
    continue
  fi
  
  # Extract everything except the first (latest) version
  OLD_VERSIONS=$(echo "$ALL_VERSIONS" | tail -n +2)
  
  for v in $OLD_VERSIONS; do
    echo "  -> Destroying old version: $v"
    gcloud secrets versions destroy "$v" --secret="$secret" --project="$PROJECT_ID" --quiet
  done
done

echo "Done!"
