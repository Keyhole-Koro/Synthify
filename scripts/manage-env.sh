#!/bin/bash
# Manage a single Synthify environment (stage|prod) against the shared
# Terraform root at terraform/environments, switching state/vars per env.
set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

usage() {
    echo "Usage: $0 <stage|prod> {up|down|plan|reset-db}"
    exit 1
}

ENVIRONMENT="${1:-}"
COMMAND="${2:-}"

case "$ENVIRONMENT" in
    stage | prod) ;;
    *) usage ;;
esac
[ -z "$COMMAND" ] && usage

TF_DIR="terraform/environments"
TFVARS="../tfvars/${ENVIRONMENT}.tfvars"
BACKEND_CONFIG="../backend/${ENVIRONMENT}.hcl"

# Single root reused across envs: -reconfigure swaps the backend cleanly.
tf_init() {
    terraform -chdir=$TF_DIR init -reconfigure -backend-config=$BACKEND_CONFIG
}

case $COMMAND in
    plan)
        log "Planning $ENVIRONMENT infrastructure..."
        tf_init
        terraform -chdir=$TF_DIR plan -var-file=$TFVARS
        ;;

    up)
        log "Initializing Terraform ($ENVIRONMENT)..."
        tf_init

        log "Applying infrastructure (Pass 1/2)..."
        terraform -chdir=$TF_DIR apply -var-file=$TFVARS -auto-approve

        PROJECT_ID=$(terraform -chdir=$TF_DIR output -raw project_id)
        ENVIRONMENT_OUT=$(terraform -chdir=$TF_DIR output -raw environment)

        log "Ensuring secrets have at least one version..."
        SECRETS=(
            "database-dsn"
            "gemini-api-key"
            "stripe-secret-key"
            "stripe-webhook-secret"
            "new-relic-license-key"
            "internal-worker-token"
        )

        for s in "${SECRETS[@]}"; do
            SECRET_NAME="synthify-$s-$ENVIRONMENT_OUT"
            if ! gcloud secrets versions list "$SECRET_NAME" --project "$PROJECT_ID" --limit=1 2>/dev/null | grep -q "ENABLED"; then
                warn "Secret $SECRET_NAME has no versions. Adding placeholder..."
                printf "placeholder-change-me" | gcloud secrets versions add "$SECRET_NAME" --project "$PROJECT_ID" --data-file=-
            fi
        done

        log "Refreshing API URL and performing Pass 2/2..."
        API_URI=$(terraform -chdir=$TF_DIR output -raw api_uri)

        terraform -chdir=$TF_DIR apply \
            -var-file=$TFVARS \
            -var="api_base_url=$API_URI" \
            -auto-approve

        success "$ENVIRONMENT environment is up!"
        terraform -chdir=$TF_DIR output
        ;;

    down)
        log "WARNING: This will destroy the $ENVIRONMENT environment."
        read -p "Type the environment name ('$ENVIRONMENT') to confirm: " -r
        echo
        if [[ "$REPLY" != "$ENVIRONMENT" ]]; then
            warn "Confirmation did not match. Aborting."
            exit 1
        fi

        tf_init

        log "Cleaning up GCS buckets..."
        BUCKET_NAME=$(terraform -chdir=$TF_DIR output -raw uploads_bucket_name 2>/dev/null || echo "")
        if [ -n "$BUCKET_NAME" ]; then
            log "Emptying bucket $BUCKET_NAME..."
            gsutil -m rm -rf "gs://$BUCKET_NAME/*" 2>/dev/null || true
        fi

        log "Destroying infrastructure..."
        terraform -chdir=$TF_DIR destroy -var-file=$TFVARS -auto-approve
        success "$ENVIRONMENT environment destroyed."
        ;;

    reset-db)
        log "WARNING: This will DROP ALL DATA in the $ENVIRONMENT database and rebuild it from db/init/."
        read -p "Type the environment name ('$ENVIRONMENT') to confirm: " -r
        echo
        if [[ "$REPLY" != "$ENVIRONMENT" ]]; then
            warn "Confirmation did not match. Aborting."
            exit 1
        fi

        tf_init

        log "Resolving project from Terraform outputs..."
        PROJECT_ID=$(terraform -chdir=$TF_DIR output -raw project_id)
        ENVIRONMENT_OUT=$(terraform -chdir=$TF_DIR output -raw environment)

        log "Fetching database connection string from Secret Manager..."
        DB_SECRET="synthify-database-dsn-$ENVIRONMENT_OUT"
        DATABASE_DSN=$(gcloud secrets versions access latest \
            --secret="$DB_SECRET" \
            --project="$PROJECT_ID")

        if [ -z "$DATABASE_DSN" ] || [ "$DATABASE_DSN" = "placeholder-change-me" ]; then
            warn "Secret $DB_SECRET has no valid connection string (got placeholder or empty). Aborting."
            exit 1
        fi

        log "Dropping and recreating public schema (and cluster-level roles)..."
        psql "$DATABASE_DSN" -v ON_ERROR_STOP=1 <<'SQL'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
DROP ROLE IF EXISTS log_viewer;
SQL

        log "Applying init scripts from db/init/ in alphabetical order..."
        for f in $(ls db/init/*.sql | sort); do
            log "  -> $f"
            psql "$DATABASE_DSN" -v ON_ERROR_STOP=1 -f "$f"
        done

        success "$ENVIRONMENT database reset and rebuilt from db/init/."
        ;;

    *)
        usage
        ;;
esac
