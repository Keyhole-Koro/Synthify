#!/usr/bin/env bash
#
# Print the NEXT_PUBLIC_* environment a compose service hands its container, as
# KEY=VALUE lines suitable for appending to $GITHUB_ENV.
#
# compose.yaml is where the local dev and e2e browser environment is defined,
# and those values are not plain constants — several interpolate port variables
# (NEXT_PUBLIC_API_BASE_URL takes BACKEND_PORT, the emulator URLs take
# FIREBASE_AUTH_PORT / FIREBASE_FIRESTORE_PORT). CI used to restate a subset of
# them by hand in web.yml, and the subset drifted: the three emulator variables
# existed only in compose. That was invisible while CI only ever ran the app
# through compose, and became a silent break the moment CI built the bundle
# itself, because NEXT_PUBLIC_* are baked in at build time — a bundle built
# without them points the Firebase SDK at real Firebase instead of the emulator.
#
# `docker compose config` resolves the interpolation and needs no daemon.
#
# Usage: scripts/compose-browser-env.sh frontend >> "$GITHUB_ENV"

set -euo pipefail

service="${1:?usage: compose-browser-env.sh <compose-service>}"
compose_file="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/compose.yaml"

resolved="$(docker compose -f "$compose_file" config --format json \
  | jq -er --arg svc "$service" '
      .services[$svc].environment
      | to_entries[]
      | select(.key | startswith("NEXT_PUBLIC_"))
      | "\(.key)=\(.value)"
    ')"

# An empty result means the service was renamed or its browser env moved. Fail
# rather than let a build quietly proceed with none of these set.
if [ -z "$resolved" ]; then
  echo "no NEXT_PUBLIC_* variables found for compose service '${service}'" >&2
  exit 1
fi

printf '%s\n' "$resolved"
