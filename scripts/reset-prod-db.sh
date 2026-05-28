#!/bin/bash
# scripts/reset-prod-db.sh
# Destructive script to reset the production database.
# Special handling for CockroachDB (DROP DATABASE CASCADE).
set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

ENVIRONMENT="prod"
TF_DIR="terraform/environments"

# Safeguard
warn "🔥 WARNING: This will DESTROY ALL DATA in the PRODUCTION database."
read -p "Type 'DESTROY PROD' to confirm: " -r
echo
if [[ "$REPLY" != "DESTROY PROD" ]]; then
    error "Confirmation did not match. Aborting."
    exit 1
fi

log "Resolving production project ID..."
PROJECT_ID=$(terraform -chdir=$TF_DIR output -raw project_id 2>/dev/null || echo "synthify-491705")

log "Fetching production database DSN from Secret Manager..."
DB_SECRET="synthify-database-dsn"
DATABASE_DSN=$(gcloud secrets versions access latest --secret="$DB_SECRET" --project="$PROJECT_ID")

if [ -z "$DATABASE_DSN" ]; then
    error "Failed to fetch DATABASE_DSN from Secret Manager."
    exit 1
fi

# psql workaround: CockroachDB Cloud DSNs often use sslmode=verify-full, 
# which requires a local root.crt. For admin tasks, sslmode=require is sufficient.
PSQL_DSN=$(echo "$DATABASE_DSN" | sed 's/sslmode=verify-full/sslmode=require/g')

# Detect CockroachDB
log "Detecting database engine..."
VERSION=$(psql "$PSQL_DSN" -At -c "SELECT version();" 2>/dev/null || echo "unknown")

if echo "$VERSION" | grep -qi "cockroachdb"; then
    log "Engine: CockroachDB detected."
    
    # Extract DB name and prepare connection to defaultdb
    DB_NAME=$(echo "$PSQL_DSN" | sed -E 's@.*/([^/?]+).*@\1@')
    DEFAULT_DSN=$(echo "$PSQL_DSN" | sed -E 's@/([^/?]+)([?]|$)@/defaultdb\2@')
    
    log "Dropping and recreating database '$DB_NAME' via defaultdb..."
    psql "$DEFAULT_DSN" -v ON_ERROR_STOP=1 <<SQL
DROP DATABASE "$DB_NAME" CASCADE;
CREATE DATABASE "$DB_NAME";
SQL
else
    log "Engine: Standard PostgreSQL (or unknown) detected."
    log "Dropping and recreating public schema..."
    psql "$PSQL_DSN" -v ON_ERROR_STOP=1 <<'SQL'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
DROP ROLE IF EXISTS monitor;
SQL
fi

log "Applying migrations from db/migrations/ in version order..."
for f in $(ls db/migrations/*.up.sql | sort); do
    log "  -> $f"
    psql "$PSQL_DSN" -v ON_ERROR_STOP=1 -f "$f"
done

success "Production database has been reset and migrations reapplied."
