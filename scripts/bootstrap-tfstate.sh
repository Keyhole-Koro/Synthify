#!/bin/bash
# Creates the GCS bucket that holds Terraform remote state for an environment.
#
# State buckets can't be Terraform-managed (chicken-and-egg), so this runs
# once per environment before `terraform init`. Idempotent: re-running on an
# existing bucket only re-asserts versioning / public-access settings.
#
# Usage:
#   scripts/bootstrap-tfstate.sh <stage|prod> <gcp-project-id> [region] [bucket-name]
#
#   region      defaults to asia-northeast1
#   bucket-name defaults to <project-id>-tfstate-<env>
#
# After it runs it writes terraform/backend/<env>.hcl for you.
set -euo pipefail

GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
log()     { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
die()     { echo -e "${RED}[ERR]${NC} $1" >&2; exit 1; }

ENV="${1:-}"
PROJECT_ID="${2:-}"
REGION="${3:-asia-northeast1}"

[ -z "$ENV" ] || [ -z "$PROJECT_ID" ] && \
  die "Usage: $0 <stage|prod> <gcp-project-id> [region] [bucket-name]"
case "$ENV" in
  stage|prod) ;;
  *) die "environment must be 'stage' or 'prod' (got '$ENV')" ;;
esac

BUCKET="${4:-${PROJECT_ID}-tfstate-${ENV}}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_FILE="${SCRIPT_DIR}/../terraform/backend/${ENV}.hcl"

command -v gcloud >/dev/null || die "gcloud not found on PATH"

log "Environment : $ENV"
log "Project     : $PROJECT_ID"
log "Region      : $REGION"
log "Bucket      : gs://$BUCKET"

if gcloud storage buckets describe "gs://$BUCKET" --project "$PROJECT_ID" >/dev/null 2>&1; then
  warn "Bucket gs://$BUCKET already exists — re-asserting settings."
else
  log "Creating bucket gs://$BUCKET ..."
  gcloud storage buckets create "gs://$BUCKET" \
    --project "$PROJECT_ID" \
    --location "$REGION" \
    --uniform-bucket-level-access \
    --public-access-prevention
fi

log "Enabling object versioning (state history / rollback) ..."
gcloud storage buckets update "gs://$BUCKET" --versioning --project "$PROJECT_ID"

# Keep noncurrent state versions for 30 days, then prune to control cost.
log "Applying lifecycle rule (prune noncurrent versions after 30 days) ..."
LIFECYCLE_TMP="$(mktemp)"
trap 'rm -f "$LIFECYCLE_TMP"' EXIT
cat > "$LIFECYCLE_TMP" <<'JSON'
{
  "rule": [
    {
      "action": { "type": "Delete" },
      "condition": { "daysSinceNoncurrentTime": 30, "isLive": false }
    }
  ]
}
JSON
gcloud storage buckets update "gs://$BUCKET" \
  --lifecycle-file="$LIFECYCLE_TMP" --project "$PROJECT_ID"

log "Writing backend config -> $BACKEND_FILE"
cat > "$BACKEND_FILE" <<EOF
bucket = "$BUCKET"
prefix = "synthify/$ENV"
EOF

success "Terraform state bucket ready for '$ENV'."
echo
echo "Next:"
echo "  cd terraform/environments"
echo "  terraform init -reconfigure -backend-config=../backend/${ENV}.hcl"
echo "  terraform apply -var-file=../tfvars/${ENV}.tfvars"
