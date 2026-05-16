#!/bin/bash
set -e

# Configuration
TF_DIR="terraform/stage"
TFVARS="../tfvars/stage.tfvars"
BACKEND_CONFIG="../backend/stage.hcl"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

usage() {
    echo "Usage: $0 {up|down|plan|reset-db}"
    exit 1
}

if [ -z "$1" ]; then
    usage
fi

COMMAND=$1

# Ensure gcloud is authenticated (optional, but helpful)
# gcloud auth application-default login

case $COMMAND in
    plan)
        log "Planning infrastructure..."
        terraform -chdir=$TF_DIR init -backend-config=$BACKEND_CONFIG
        terraform -chdir=$TF_DIR plan -var-file=$TFVARS
        ;;

    up)
        log "Initializing Terraform..."
        terraform -chdir=$TF_DIR init -backend-config=$BACKEND_CONFIG

        log "Applying infrastructure (Pass 1/2)..."
        terraform -chdir=$TF_DIR apply -var-file=$TFVARS -auto-approve

        # Initialize secrets if they don't have versions
        PROJECT_ID=$(terraform -chdir=$TF_DIR output -raw project_id)
        ENVIRONMENT=$(terraform -chdir=$TF_DIR output -raw environment)
        
        log "Ensuring secrets have at least one version..."
        SECRETS=(
            "database-url"
            "gemini-api-key"
            "stripe-secret-key"
            "stripe-webhook-secret"
            "new-relic-license-key"
            "internal-worker-token"
        )

        for s in "${SECRETS[@]}"; do
            SECRET_NAME="synthify-$s-$ENVIRONMENT"
            if ! gcloud secrets versions list "$SECRET_NAME" --project "$PROJECT_ID" --limit=1 2>/dev/null | grep -q "ENABLED"; then
                warn "Secret $SECRET_NAME has no versions. Adding placeholder..."
                printf "placeholder-change-me" | gcloud secrets versions add "$SECRET_NAME" --project "$PROJECT_ID" --data-file=-
            fi
        done

        log "Refreshing API URL and performing Pass 2/2..."
        API_URI=$(terraform -chdir=$TF_DIR output -raw api_uri)
        
        # Re-apply with api_base_url set
        terraform -chdir=$TF_DIR apply \
            -var-file=$TFVARS \
            -var="api_base_url=$API_URI" \
            -auto-approve

        success "Stage environment is up!"
        terraform -chdir=$TF_DIR output
        ;;

    down)
        log "WARNING: This will destroy the stage environment."
        read -p "Are you sure? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi

        log "Cleaning up GCS buckets..."
        # We try to find the bucket name from TF output
        BUCKET_NAME=$(terraform -chdir=$TF_DIR output -raw uploads_bucket_name 2>/dev/null || echo "")
        if [ -n "$BUCKET_NAME" ]; then
            log "Emptying bucket $BUCKET_NAME..."
            gsutil -m rm -rf "gs://$BUCKET_NAME/*" 2>/dev/null || true
        fi

        log "Destroying infrastructure..."
        terraform -chdir=$TF_DIR destroy -var-file=$TFVARS -auto-approve
        success "Stage environment destroyed."
        ;;

    reset-db)
        log "WARNING: This will DROP ALL DATA in the stage database and rebuild it from db/init/."
        read -p "Are you sure? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi

        log "Resolving project from Terraform outputs..."
        PROJECT_ID=$(terraform -chdir=$TF_DIR output -raw project_id)
        ENVIRONMENT=$(terraform -chdir=$TF_DIR output -raw environment)

        log "Fetching database connection string from Secret Manager..."
        DB_SECRET="synthify-database-url-$ENVIRONMENT"
        DATABASE_URL=$(gcloud secrets versions access latest \
            --secret="$DB_SECRET" \
            --project="$PROJECT_ID")

        if [ -z "$DATABASE_URL" ] || [ "$DATABASE_URL" = "placeholder-change-me" ]; then
            warn "Secret $DB_SECRET has no valid connection string (got placeholder or empty). Aborting."
            exit 1
        fi

        log "Dropping and recreating public schema (and cluster-level roles)..."
        # DROP SCHEMA only clears schema-scoped objects. Roles live at the
        # cluster level, so drop them explicitly to keep reset-db idempotent.
        psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
DROP ROLE IF EXISTS log_viewer;
SQL

        log "Applying init scripts from db/init/ in alphabetical order..."
        for f in $(ls db/init/*.sql | sort); do
            log "  -> $f"
            psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"
        done

        success "Stage database reset and rebuilt from db/init/."
        ;;

    *)
        usage
        ;;
esac
