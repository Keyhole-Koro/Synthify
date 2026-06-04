#!/bin/sh
set -eu

HOST="${POSTGRES_HOST:-127.0.0.1:${POSTGRES_PORT:-5432}}"
DB="${POSTGRES_DB:-synthify}"
APP_USER="${POSTGRES_APP_USER:-synthify}"
STATE_TABLE="${POSTGRES_INIT_STATE_TABLE:-dev_init_state}"
STATE_FILE="/tmp/postgres-init-state.csv"

sql_root() {
  cockroach sql --insecure --host="$HOST" --user=root "$@"
}

sql_db() {
  cockroach sql --insecure --host="$HOST" --user=root --database="$DB" "$@"
}

checksum_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    cksum "$1" | awk '{print $1 "-" $2}'
  fi
}

stored_checksum() {
  awk -F, -v key="$1" 'NR > 1 && $1 == key { print $2; exit }' "$STATE_FILE" | tr -d '\r'
}

record_checksum() {
  sql_db -e "UPSERT INTO ${STATE_TABLE} (path, checksum, applied_at) VALUES ('$1', '$2', now());"
  awk -F, -v key="$1" 'NR == 1 || $1 != key' "$STATE_FILE" > "${STATE_FILE}.tmp"
  printf '%s,%s\n' "$1" "$2" >> "${STATE_FILE}.tmp"
  mv "${STATE_FILE}.tmp" "$STATE_FILE"
}

apply_tracked_file() {
  key="$1"
  file="$2"
  user="$3"
  checksum="$(checksum_file "$file")"
  stored="$(stored_checksum "$key")"

  if [ "$stored" = "$checksum" ]; then
    echo "Skipping unchanged $key"
    return 0
  fi

  echo "Applying $key..."
  cockroach sql --insecure --host="$HOST" --user="$user" --database="$DB" < "$file"
  record_checksum "$key" "$checksum"
}

echo "Ensuring local database and users exist..."
sql_root -e "CREATE USER IF NOT EXISTS ${APP_USER}; CREATE DATABASE IF NOT EXISTS ${DB}; GRANT ALL ON DATABASE ${DB} TO ${APP_USER};"

sql_db -e "
  CREATE TABLE IF NOT EXISTS ${STATE_TABLE} (
    path STRING PRIMARY KEY,
    checksum STRING NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
"
sql_db --format=csv -e "SELECT path, checksum FROM ${STATE_TABLE};" > "$STATE_FILE"

for f in /migrations/*.up.sql; do
  [ -f "$f" ] || continue
  apply_tracked_file "migration:$(basename "$f")" "$f" root
done

echo "Granting ${APP_USER} access to migrated objects..."
sql_db -e "GRANT ALL ON SCHEMA public TO ${APP_USER}; GRANT ALL ON ALL TABLES IN SCHEMA public TO ${APP_USER}; GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO ${APP_USER};"

for f in /seed/*.sql; do
  [ -f "$f" ] || continue
  apply_tracked_file "seed:$(basename "$f")" "$f" "$APP_USER"
done

echo "Postgres local init complete."
