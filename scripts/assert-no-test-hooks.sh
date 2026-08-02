#!/usr/bin/env bash
#
# Fail if a built artifact contains a test-only affordance.
#
# apps/web exposes window.__synthifyE2E so Playwright can establish a real
# Firebase session — the app's only sign-in path is Google OAuth, which a test
# runner cannot drive, so the hook is the bridge to the emulator's
# email/password path. It signs a user in from anything that can reach the
# global, which is fine in development and unacceptable in a deployed bundle.
#
# firebase.ts gates it on NODE_ENV === 'development' and its comment states the
# hook "is never present in a production bundle/environment". That was an
# intention with nothing checking it. This makes it a property of the artifact.
#
# Usage: scripts/assert-no-test-hooks.sh apps/web/out
set -euo pipefail

root="${1:?usage: assert-no-test-hooks.sh <build-output-dir>}"

if [ ! -d "$root" ]; then
  echo "no build output at ${root}" >&2
  exit 1
fi

# Property names reached through a global object survive minification, so a
# literal search is enough — the bundler cannot rename window.__synthifyE2E
# without breaking the caller it was written for.
markers=(
  __synthifyE2E
)

failed=0
for marker in "${markers[@]}"; do
  # -r over the whole tree: the hook could land in any chunk, and which chunk is
  # a build detail that must not be part of this check.
  if hits="$(grep -rl --binary-files=without-match -- "$marker" "$root" 2>/dev/null)" && [ -n "$hits" ]; then
    echo "::error::test hook '${marker}' found in the build output" >&2
    printf '  %s\n' $hits >&2
    failed=1
  fi
done

if [ "$failed" -ne 0 ]; then
  cat >&2 <<'EOF'

This artifact is about to be deployed. A test hook in it means the gate that is
supposed to keep it out of shipped builds did not hold — check NODE_ENV at build
time and the condition in apps/web/src/lib/firebase.ts before releasing.
EOF
  exit 1
fi

echo "no test hooks in ${root}"
