#!/usr/bin/env bash
set -euo pipefail

TIMEOUT_SECONDS="${SMOKE_TEST_TIMEOUT_SECONDS:-15}"
RETRY_COUNT="${SMOKE_TEST_RETRY_COUNT:-2}"

# Deploy smoke test entrypoint. Keep these checks low-side-effect: they run
# immediately after stage/prod deploys and are allowed to fail the workflow.
usage() {
  cat <<'EOF'
Usage:
  API_BASE_URL=https://... READINESS_API_KEY=... scripts/smoke-test.sh api
  WORKER_BASE_URL=https://... READINESS_API_KEY=... scripts/smoke-test.sh worker
  FRONTEND_URL=https://... scripts/smoke-test.sh frontend

You can pass multiple targets:
  API_BASE_URL=https://... WORKER_BASE_URL=https://... READINESS_API_KEY=... scripts/smoke-test.sh api worker
EOF
}

die() {
  echo "smoke-test: $*" >&2
  exit 1
}

require_env() {
  local name="$1"
  local value="${!name:-}"
  if [[ -z "$value" ]]; then
    die "$name is required"
  fi
  printf '%s' "${value%/}"
}

check_url() {
  local label="$1"
  local url="$2"
  shift 2
  local body
  local status

  body="$(mktemp)"
  status="$(
    curl \
      --silent \
      --show-error \
      --location \
      --connect-timeout 5 \
      --max-time "$TIMEOUT_SECONDS" \
      --retry "$RETRY_COUNT" \
      --retry-delay 2 \
      --output "$body" \
      --write-out '%{http_code}' \
      "$@" \
      "$url"
  )" || {
    local exit_code=$?
    echo "smoke-test: $label failed to reach $url (curl exit $exit_code)" >&2
    rm -f "$body"
    return "$exit_code"
  }

  if [[ "$status" != 2* ]]; then
    echo "smoke-test: $label returned HTTP $status for $url" >&2
    sed -n '1,40p' "$body" >&2 || true
    rm -f "$body"
    return 1
  fi

  rm -f "$body"
  echo "smoke-test: $label OK ($status)"
}

check_readiness_url() {
  local label="$1"
  local url="$2"
  local key

  key="$(require_env READINESS_API_KEY)"
  check_url "$label" "$url" --header "X-Synthify-Readiness-Key: $key"
}

check_status() {
  local label="$1"
  local want="$2"
  local url="$3"
  shift 3
  local body
  local status

  body="$(mktemp)"
  status="$(
    curl \
      --silent \
      --show-error \
      --location \
      --connect-timeout 5 \
      --max-time "$TIMEOUT_SECONDS" \
      --retry "$RETRY_COUNT" \
      --retry-delay 2 \
      --output "$body" \
      --write-out '%{http_code}' \
      "$@" \
      "$url"
  )" || {
    local exit_code=$?
    echo "smoke-test: $label failed to reach $url (curl exit $exit_code)" >&2
    rm -f "$body"
    return "$exit_code"
  }

  if [[ "$status" != "$want" ]]; then
    echo "smoke-test: $label returned HTTP $status for $url; expected $want" >&2
    sed -n '1,40p' "$body" >&2 || true
    rm -f "$body"
    return 1
  fi

  rm -f "$body"
  echo "smoke-test: $label OK ($status)"
}

check_frontend() {
  local url="$1"
  local body
  local status

  body="$(mktemp)"
  status="$(
    curl \
      --silent \
      --show-error \
      --location \
      --connect-timeout 5 \
      --max-time "$TIMEOUT_SECONDS" \
      --retry "$RETRY_COUNT" \
      --retry-delay 2 \
      --output "$body" \
      --write-out '%{http_code}' \
      "$url"
  )" || {
    local exit_code=$?
    echo "smoke-test: frontend failed to reach $url (curl exit $exit_code)" >&2
    rm -f "$body"
    return "$exit_code"
  }

  if [[ "$status" != 2* ]]; then
    echo "smoke-test: frontend returned HTTP $status for $url" >&2
    sed -n '1,40p' "$body" >&2 || true
    rm -f "$body"
    return 1
  fi

  if ! grep -Eiq '<html|<!doctype html|__next|<script' "$body"; then
    echo "smoke-test: frontend response did not look like an HTML app shell" >&2
    sed -n '1,40p' "$body" >&2 || true
    rm -f "$body"
    return 1
  fi

  rm -f "$body"
  echo "smoke-test: frontend OK ($status)"
}

if [[ "$#" -eq 0 ]]; then
  usage >&2
  exit 2
fi

for target in "$@"; do
  case "$target" in
    api)
      api_base_url="$(require_env API_BASE_URL)"
      check_url "api health" "$api_base_url/health"
      check_readiness_url "api readiness" "$api_base_url/health?ready=1"
      check_status \
        "api auth gate" \
        "401" \
        "$api_base_url/synthify.app.v1.WorkspaceService/ListWorkspaces" \
        --request POST \
        --header 'Content-Type: application/json' \
        --data '{}'
      ;;
    worker)
      worker_base_url="$(require_env WORKER_BASE_URL)"
      check_url "worker health" "$worker_base_url/health"
      check_readiness_url "worker readiness" "$worker_base_url/health?ready=1"
      check_status \
        "worker connect route" \
        "400" \
        "$worker_base_url/synthify.worker.v1.WorkerService/GenerateExecutionPlan" \
        --request POST \
        --header 'Content-Type: application/json' \
        --data '{}'
      ;;
    frontend)
      frontend_url="$(require_env FRONTEND_URL)"
      check_frontend "$frontend_url"
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      usage >&2
      die "unknown target: $target"
      ;;
  esac
done
