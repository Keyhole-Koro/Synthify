#!/usr/bin/env bash
# L2 of the client load test: put N real tree_items in the database so the
# whole path — SQL, GetTree, the wire, the client — can be measured end to end.
# The L0 bench and the L1 Playwright run both inject their trees in the
# frontend, which is what makes them cheap and reproducible; it also means
# neither of them can see API or query cost. This script is how you see it.
#
# See docs/improvements/client-item-load-testing.md.
#
# Usage:
#   scripts/seed_tree_items.sh <workspace_id> [total_items] [branching] [depth] [content_bytes]
#
# Example — 5000 nodes, 6 children each, 5 levels, ~2 KiB of HTML per node:
#   scripts/seed_tree_items.sh ws_seed_1 5000 6 5 2048
#
# Re-running replaces the previously seeded nodes for that workspace: every row
# it creates is prefixed `loadtest_`, and only those rows are deleted. Nodes
# produced by real jobs are left alone.
set -euo pipefail

WORKSPACE_ID="${1:-}"
TOTAL_ITEMS="${2:-5000}"
BRANCHING="${3:-6}"
DEPTH="${4:-5}"
CONTENT_BYTES="${5:-2048}"

if [ -z "$WORKSPACE_ID" ]; then
  echo "usage: $0 <workspace_id> [total_items] [branching] [depth] [content_bytes]" >&2
  exit 2
fi

COMPOSE_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/compose.yaml"
HOST="127.0.0.1:${POSTGRES_PORT:-5432}"

sql() {
  docker compose -f "$COMPOSE_FILE" exec -T postgres \
    cockroach sql --insecure --host="$HOST" --database=synthify "$@"
}

exists=$(sql --format=csv -e "SELECT count(*) FROM workspaces WHERE workspace_id = '${WORKSPACE_ID}';" | tail -n 1)
if [ "$exists" != "1" ]; then
  echo "workspace ${WORKSPACE_ID} not found — create it first (or use a seed workspace such as ws_seed_1)" >&2
  exit 1
fi

echo "Removing previously seeded load-test nodes from ${WORKSPACE_ID}..."
sql -e "DELETE FROM tree_items WHERE workspace_id = '${WORKSPACE_ID}' AND id LIKE 'loadtest_%';"

echo "Seeding ${TOTAL_ITEMS} nodes (branching=${BRANCHING}, depth=${DEPTH}, content=${CONTENT_BYTES}B)..."

# Node ids are b-ary heap indices: the children of node i are i*b+1 .. i*b+b,
# so the parent of node j is (j-1)/b. Truncating at total_items therefore
# leaves the tree balanced — the same shape mockTreeGenerator.ts builds, which
# is what makes L1 and L2 numbers comparable.
#
# tree_items.parent_id is self-referencing, so parents must land before their
# children. Heap order is breadth-first, so inserting ORDER BY idx satisfies
# that in one statement.
sql <<SQL
WITH RECURSIVE nodes(idx, lvl) AS (
    SELECT 0::INT8, 0::INT8
  UNION ALL
    SELECT nodes.idx * ${BRANCHING}::INT8 + k, nodes.lvl + 1
    FROM nodes, generate_series(1, ${BRANCHING}::INT8) AS k
    WHERE nodes.idx * ${BRANCHING}::INT8 + k < ${TOTAL_ITEMS}::INT8
      AND nodes.lvl + 1 <= ${DEPTH}::INT8
)
INSERT INTO tree_items (
  id, workspace_id, parent_id, title, level, description, content,
  override_css, created_by, governance_state, last_mutation_job_id,
  cross_document, created_at, updated_at
)
SELECT
  'loadtest_${WORKSPACE_ID}_' || idx::STRING,
  '${WORKSPACE_ID}',
  CASE WHEN idx = 0 THEN NULL
       ELSE 'loadtest_${WORKSPACE_ID}_' || ((idx - 1) / ${BRANCHING}::INT8)::STRING END,
  CASE WHEN idx = 0 THEN 'Load test root' ELSE 'Load test node ' || idx::STRING END,
  lvl,
  'Seeded by scripts/seed_tree_items.sh',
  '<h2>Load test node ' || idx::STRING || '</h2>'
    || repeat('<p>knowledge node synthesis evidence projection expansion payload</p>',
              GREATEST(1, ${CONTENT_BYTES}::INT8 / 60)),
  '', 'load-test', 'system_generated', '',
  false, now(), now()
FROM nodes
ORDER BY idx;
SQL

sql -e "SELECT count(*) AS seeded_items, sum(length(content)) AS content_bytes FROM tree_items WHERE workspace_id = '${WORKSPACE_ID}' AND id LIKE 'loadtest_%';"

cat <<EOF

Seeded. To measure the real GetTree path against this workspace:

  1. Open the app, log in as a user with access to ${WORKSPACE_ID}, and open it.
  2. In DevTools, watch the GetTree response size and timing on the Network tab —
     connect-web is on the JSON codec, so the transfer includes every node's
     content, and decode cost scales with it.
  3. Compare against the L0 bench numbers (bun run bench) to see how much of the
     wall time is transport/decode versus client-side work.

To remove the seeded nodes again:

  docker compose exec -T postgres cockroach sql --insecure --host=${HOST} --database=synthify \\
    -e "DELETE FROM tree_items WHERE workspace_id = '${WORKSPACE_ID}' AND id LIKE 'loadtest_%';"
EOF
