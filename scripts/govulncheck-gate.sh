#!/usr/bin/env bash
#
# Run govulncheck on the root Go module and decide whether CI should fail.
#
# Without a base ref (the weekly schedule, pushes to main), any vulnerability
# that the code reaches fails the run: that is how a newly published advisory
# gets noticed.
#
# With a base ref (pull requests), the base commit is scanned too and only the
# reachable vulnerabilities that the base does not have fail the run. A pull
# request is not blocked by findings that were already on its base branch, and
# one that fixes them goes green; one that adds a vulnerable dependency, or a
# call into vulnerable code, does not.
#
# Every reachable vulnerability of HEAD is reported either way, in the text
# report and in $GITHUB_STEP_SUMMARY when it is set.
#
# Needs govulncheck and jq on PATH. The same Go toolchain scans both commits,
# so the standard library is judged the same way on each side.
#
# Usage: scripts/govulncheck-gate.sh [base-ref]
set -euo pipefail
# sort and comm must agree on the order.
export LC_ALL=C

base_ref="${1:-}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

summary="${GITHUB_STEP_SUMMARY:-/dev/null}"

# reachable_ids DIR prints the IDs of the vulnerabilities that the code in DIR
# reaches (symbol-level findings), one per line. In JSON mode govulncheck exits
# 0 whatever it finds, so a non-zero exit is an error of the scan itself.
reachable_ids() {
  govulncheck -C "$1" -format json ./... \
    | jq -r 'select(.finding != null and .finding.trace[0].function != null) | .finding.osv' \
    | sort -u
}

# The readable report for HEAD. Exit status 3 means "vulnerabilities found";
# anything else non-zero is a failed scan.
set +e
govulncheck -C "$repo_root" ./... >"$work/report.txt" 2>&1
status=$?
set -e
cat "$work/report.txt"
if [ "$status" -ne 0 ] && [ "$status" -ne 3 ]; then
  echo "::error::govulncheck failed (exit $status)"
  exit "$status"
fi

reachable_ids "$repo_root" >"$work/head.txt"

if [ -z "$base_ref" ]; then
  cp "$work/head.txt" "$work/new.txt"
  scope="reachable"
else
  mkdir "$work/base"
  git -C "$repo_root" archive "$base_ref" | tar -x -C "$work/base"
  reachable_ids "$work/base" >"$work/base.txt"
  comm -23 "$work/head.txt" "$work/base.txt" >"$work/new.txt"
  scope="reachable and not on the base ($base_ref)"
fi

{
  echo '## govulncheck'
  echo
  if [ -s "$work/head.txt" ]; then
    echo "Reachable: $(paste -sd' ' "$work/head.txt")"
  else
    echo 'Reachable: none'
  fi
  if [ -n "$base_ref" ]; then
    if [ -s "$work/new.txt" ]; then
      echo
      echo "New in this change: $(paste -sd' ' "$work/new.txt")"
    else
      echo
      echo 'New in this change: none'
    fi
  fi
  echo
  echo '```'
  cat "$work/report.txt"
  echo '```'
} >>"$summary"

if [ -s "$work/new.txt" ]; then
  while read -r id; do
    echo "::error::$id is $scope: https://pkg.go.dev/vuln/$id"
  done <"$work/new.txt"
  exit 1
fi
echo "No vulnerabilities that are $scope."
