#!/bin/bash
# Apply db/migrations/*.up.sql to the target environment's database using
# golang-migrate. Forward-only: each numbered migration runs at most once and
# is recorded in the schema_migrations table.
#
# Seed data lives in db/seed/seed.sql and is intentionally not run here —
# real environments must never receive demo accounts/workspaces/documents.
#
# Baselining existing databases:
#   stage and prod were originally populated by raw psql against db/init/*.sql,
#   so they hold the schema but have no schema_migrations row. On the first
#   run after the cutover this script detects that state (accounts table
#   exists, schema_migrations does not) and stamps the database with the
#   current head via `migrate force` before running `up`. After that the
#   normal up-only flow takes over.
#
# Usage:
#   PROJECT_ID=synthify-491705 bash scripts/apply-db-init.sh
#
# Requires:
#   - gcloud authenticated with access to secret synthify-database-dsn
#   - migrate CLI on PATH (https://github.com/golang-migrate/migrate)
#   - psql on PATH (only used for the baseline detection probe)
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
if [ -z "$PROJECT_ID" ]; then
    echo "PROJECT_ID is required (the GCP project hosting synthify-database-dsn)" >&2
    exit 1
fi

for bin in migrate psql gcloud; do
    if ! command -v "$bin" >/dev/null 2>&1; then
        echo "required binary '$bin' not found on PATH" >&2
        exit 1
    fi
done

DB_SECRET="synthify-database-dsn"

echo "Fetching $DB_SECRET from project $PROJECT_ID..."
DATABASE_DSN="$(gcloud secrets versions access latest \
    --secret="$DB_SECRET" \
    --project="$PROJECT_ID")"

if [ -z "$DATABASE_DSN" ] || [ "$DATABASE_DSN" = "placeholder-change-me" ]; then
    echo "Secret $DB_SECRET has no valid connection string (got placeholder or empty)." >&2
    exit 1
fi

# CockroachDB Cloud presents a Let's Encrypt cert. CI runners have the system
# trust store but no ~/.postgresql/root.crt, so a DSN that pins sslrootcert to
# a file path would fail. Rewrite sslrootcert to "system" when present so
# libpq (and migrate's drivers) use the OS bundle.
if echo "$DATABASE_DSN" | grep -q "sslrootcert="; then
    DATABASE_DSN="$(echo "$DATABASE_DSN" | sed -E 's#sslrootcert=[^&]*#sslrootcert=system#')"
fi

# golang-migrate's default postgres driver uses pg_advisory_lock(), which
# CockroachDB does not implement. Switch the scheme to cockroachdb:// so
# migrate picks its CockroachDB-aware driver (no advisory lock required).
# psql still needs the plain postgres:// form, so keep both.
MIGRATE_DSN="$(echo "$DATABASE_DSN" | sed -E 's#^postgres(ql)?://#cockroachdb://#')"

# psql on a stock GitHub runner has no ~/.postgresql/root.crt, so an
# sslmode=verify-full DSN fails connect with the system trust store unset
# in libpq's PG_SYSCONFDIR. Probes run in a sub-shell that swallows errors,
# which would silently disable baseline detection. Downgrade to require for
# the probe path only — it still encrypts, just skips cert verification, and
# this script only reads schema_migrations/accounts metadata.
PSQL_DSN="$(echo "$DATABASE_DSN" | sed -E 's#sslmode=[^&]*#sslmode=require#')"

cd "$(dirname "$0")/.."

MIGRATIONS_DIR="db/migrations"
if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo "$MIGRATIONS_DIR not found" >&2
    exit 1
fi

# Highest numbered up-migration = head version we baseline existing dbs to.
HEAD_VERSION="$(ls "$MIGRATIONS_DIR" | grep -oE '^[0-9]+' | sort -n | tail -1)"
if [ -z "$HEAD_VERSION" ]; then
    echo "no migrations found in $MIGRATIONS_DIR" >&2
    exit 1
fi
# migrate stores versions as plain integers (1, 2, ...) not zero-padded.
HEAD_VERSION="$((10#$HEAD_VERSION))"

# Fail loudly if psql cannot reach the database at all. The probes below
# swallow per-query errors so they degrade to "table missing" gracefully,
# but a connection failure must not be masked the same way — that path
# led to baseline/dirty detection being silently skipped on stage.
if ! psql "$PSQL_DSN" -At -c "SELECT 1" >/dev/null 2>&1; then
    echo "psql could not connect to the database — schema probes will not work." >&2
    echo "Re-run with sslmode/sslrootcert that the runner's libpq accepts." >&2
    exit 1
fi

probe() {
    psql "$PSQL_DSN" -At -c "$1" 2>/dev/null || true
}

has_schema_migrations="$(probe "SELECT to_regclass('public.schema_migrations') IS NOT NULL")"
has_accounts="$(probe "SELECT to_regclass('public.accounts') IS NOT NULL")"
echo "Probe results: schema_migrations=${has_schema_migrations:-<empty>}, accounts=${has_accounts:-<empty>}"

if [ "$has_schema_migrations" != "t" ] && [ "$has_accounts" = "t" ]; then
    echo "Existing schema detected without schema_migrations table — baselining to version $HEAD_VERSION."
    migrate -path "$MIGRATIONS_DIR" -database "$MIGRATE_DSN" force "$HEAD_VERSION"
fi

# If a previous run failed mid-migration, schema_migrations.dirty=true and
# migrate refuses to continue. Roll the marker back to the last successfully
# applied version and let `up` retry. We treat the version *before* the
# dirty one as "successfully applied" because the failed migration is
# expected to be rerun once the underlying SQL is fixed.
if [ "$has_schema_migrations" = "t" ]; then
    dirty_version="$(probe "SELECT version::text FROM schema_migrations WHERE dirty = true LIMIT 1")"
    echo "Dirty probe: version='${dirty_version:-<none>}'"
    if [ -n "$dirty_version" ]; then
        prev=$((dirty_version - 1))
        if [ "$prev" -lt 0 ]; then prev=0; fi
        echo "schema_migrations is dirty at version $dirty_version — forcing back to $prev so up can retry."
        migrate -path "$MIGRATIONS_DIR" -database "$MIGRATE_DSN" force "$prev"
    fi
fi

echo "Running migrate up..."
migrate -path "$MIGRATIONS_DIR" -database "$MIGRATE_DSN" up

echo "Current migration state:"
migrate -path "$MIGRATIONS_DIR" -database "$MIGRATE_DSN" version
