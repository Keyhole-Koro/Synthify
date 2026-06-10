#!/usr/bin/env bash
# Local docker-compose DB bring-up. Applies db/migrations/*.up.sql with
# golang-migrate (version-tracked in schema_migrations, each migration applied
# at most once), then loads dev seed data. This is the local-insecure sibling
# of scripts/apply-db-migrations.sh — same forward-only semantics, but it talks
# to the compose CockroachDB over an insecure connection instead of pulling a
# DSN from Secret Manager.
#
# Why golang-migrate and not a raw `for f in *.up.sql` loop: forward-only
# migrations such as 0018 DROP columns that earlier files (0008) still index.
# A loop that replays every file on each boot re-runs 0008's index-on-`kind`
# after 0018 has dropped `kind`, which fails with "column \"kind\" does not
# exist". Version tracking runs each migration exactly once and avoids that.
set -euo pipefail

HOST="127.0.0.1:${POSTGRES_PORT:-5432}"
MIGRATIONS_DIR="/migrations"
MIGRATE_VERSION="${MIGRATE_VERSION:-v4.17.1}"

# golang-migrate is not in the cockroach image; fetch the static binary once
# per run (same source as .github/workflows/deploy-backend.yml).
if ! command -v migrate >/dev/null 2>&1; then
  echo "Installing golang-migrate ${MIGRATE_VERSION}..."
  curl -fsSL "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" \
    | tar -xz -C /tmp migrate
  install -m 0755 /tmp/migrate /usr/local/bin/migrate
fi

sql() { cockroach sql --insecure --host="$HOST" "$@"; }

echo "Ensuring synthify role and database..."
sql -e "CREATE USER IF NOT EXISTS synthify; CREATE DATABASE IF NOT EXISTS synthify; GRANT ALL ON DATABASE synthify TO synthify;"

# golang-migrate's default postgres driver needs pg_advisory_lock(), which
# CockroachDB does not implement. The cockroachdb:// scheme selects its
# lock-free driver.
MIGRATE_DSN="cockroachdb://root@${HOST}/synthify?sslmode=disable"
migrate_cli() { migrate -path "$MIGRATIONS_DIR" -database "$MIGRATE_DSN" "$@"; }

# Head version = highest numbered migration. migrate stores versions as plain
# integers (20, not 0020).
HEAD_VERSION="$(ls "$MIGRATIONS_DIR" | grep -oE '^[0-9]+' | sort -n | tail -1)"
HEAD_VERSION="$((10#$HEAD_VERSION))"

# --format=csv prints a header row then the value; tail -n +2 drops the header
# so an empty result set yields an empty string (not the column name).
probe() { sql --database=synthify --format=csv -e "$1" 2>/dev/null | tail -n +2 | tr -d '[:space:]'; }

has_accounts="$(probe "SELECT to_regclass('public.accounts') IS NOT NULL")"
applied_version="$(probe "SELECT version::text FROM schema_migrations LIMIT 1")"

# Volumes created by the old raw-replay init hold the full schema but have no
# recorded version (no schema_migrations table, or the table exists but is
# empty). Stamp them at head so migrate does not try to re-run 0001+ against
# tables that already exist. Mirror of the prod baseline in
# scripts/apply-db-migrations.sh.
if [ "$has_accounts" = "t" ] && [ -z "$applied_version" ]; then
  echo "Existing schema with no recorded migration version — baselining to $HEAD_VERSION."
  migrate_cli force "$HEAD_VERSION"
fi

# A migration that failed mid-run leaves schema_migrations.dirty=true and
# migrate refuses to continue. Roll the marker back one version so `up` can
# retry the fixed SQL.
dirty_version="$(probe "SELECT version::text FROM schema_migrations WHERE dirty = true LIMIT 1")"
if [ -n "$dirty_version" ]; then
  prev=$((dirty_version - 1))
  [ "$prev" -lt 0 ] && prev=0
  echo "schema_migrations is dirty at $dirty_version — forcing back to $prev so up can retry."
  migrate_cli force "$prev"
fi

echo "Running migrate up..."
migrate_cli up
migrate_cli version

echo "Granting synthify access to migrated objects..."
sql --database=synthify -e "GRANT ALL ON SCHEMA public TO synthify; GRANT ALL ON ALL TABLES IN SCHEMA public TO synthify; GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO synthify;"

for f in /seed/*.sql; do
  [ -f "$f" ] || continue
  echo "Loading seed $f..."
  cockroach sql --insecure --host="$HOST" --user=synthify --database=synthify < "$f"
done
