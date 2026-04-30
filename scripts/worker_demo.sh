#!/usr/bin/env bash
# worker_demo.sh — seed ドキュメントを worker に流して結果を観察する
#
# 前提:
#   docker compose up -d
#   bash scripts/seed_gcs.sh  (初回のみ)
#   GEMINI_API_KEY が .env または環境変数に設定されていること
#
# 使い方:
#   bash scripts/worker_demo.sh [doc_llm_1|doc_llm_2|doc_llm_3]

set -uo pipefail

DOC_ID="${1:-doc_llm_1}"
WORKER_URL="${WORKER_URL:-http://localhost:8081}"
WORKER_TOKEN="${INTERNAL_WORKER_TOKEN:-dev-worker-token}"
PG_URL="${DATABASE_URL:-postgres://synthify:synthify@localhost:5432/synthify?sslmode=disable}"
# ホスト側（疎通確認用）とコンテナ内部（workerが実際に使う）でホスト名が異なる
GCS_UPLOAD_URL_BASE="${GCS_UPLOAD_URL_BASE:-http://localhost:4443/storage/v1/b/synthify-uploads/o}"
GCS_INTERNAL_URL_BASE="${GCS_INTERNAL_URL_BASE:-http://gcs:4443/storage/v1/b/synthify-uploads/o}"
PSQL="psql ${PG_URL} --tuples-only --no-align -q"

# ── ドキュメントメタデータを取得 ────────────────────────────────────────────
WS_ID=$(${PSQL} -c "SELECT workspace_id FROM documents WHERE document_id = '${DOC_ID}';" | tr -d ' ')
FILENAME=$(${PSQL} -c "SELECT filename FROM documents WHERE document_id = '${DOC_ID}';" | tr -d ' ')

if [[ -z "${WS_ID}" ]]; then
  echo "ERROR: document '${DOC_ID}' が DB にありません。"
  echo "  1. docker compose up -d postgres"
  echo "  2. DB が起動したら 002_seed.sql が自動適用されます"
  exit 1
fi

# 疎通確認はホスト側URL、RPCで渡すのはコンテナ内部URL
FILE_URI_HOST="${GCS_UPLOAD_URL_BASE}/${WS_ID}%2F${DOC_ID}?alt=media"
FILE_URI_INTERNAL="${GCS_INTERNAL_URL_BASE}/${WS_ID}%2F${DOC_ID}?alt=media"

echo ""
echo "══════════════════════════════════════════════════════"
echo "  Synthify Worker Demo"
echo "  document : ${DOC_ID} (${FILENAME})"
echo "  workspace: ${WS_ID}"
echo "  file_uri : ${FILE_URI_HOST}"
echo "══════════════════════════════════════════════════════"

# GCS にファイルが存在するか確認
HTTP_STATUS=$(curl -so /dev/null -w "%{http_code}" "${FILE_URI_HOST}")
if [[ "${HTTP_STATUS}" != "200" ]]; then
  echo ""
  echo "ERROR: GCS にファイルがありません (HTTP ${HTTP_STATUS})"
  echo "  bash scripts/seed_gcs.sh を先に実行してください。"
  exit 1
fi

# ── before スナップショット ───────────────────────────────────────────────────
BEFORE_CHUNKS=$(${PSQL} -c "SELECT COUNT(*) FROM document_chunks WHERE document_id = '${DOC_ID}';" | tr -d ' ')
BEFORE_EMBEDDED=$(${PSQL} -c "SELECT COUNT(*) FROM document_chunks WHERE document_id = '${DOC_ID}' AND embedding IS NOT NULL;" | tr -d ' ')
BEFORE_ITEMS=$(${PSQL} -c "SELECT COUNT(*) FROM tree_items WHERE workspace_id = '${WS_ID}' AND created_by = 'llm_worker';" | tr -d ' ')

echo ""
echo "── Before ──────────────────────────────────────────"
printf "  chunks    : %s\n" "${BEFORE_CHUNKS}"
printf "  embedded  : %s\n" "${BEFORE_EMBEDDED}"
printf "  tree_items: %s\n" "${BEFORE_ITEMS}"

# tree_id = workspace_id (trees テーブルは存在しない。GetOrCreateTree が tree_items でルートを管理する)
TREE_ID="${WS_ID}"

# ── job と capability を作成 ─────────────────────────────────────────────────
JOB_ID="demo-$(date +%s)-${DOC_ID}"

psql "${PG_URL}" -q -c "
  INSERT INTO document_processing_jobs
    (job_id, document_id, workspace_id, job_type, status, current_stage, created_at, updated_at)
  VALUES
    ('${JOB_ID}', '${DOC_ID}', '${WS_ID}', 'JOB_TYPE_PROCESS_DOCUMENT', 'queued', '', NOW(), NOW());
"

psql "${PG_URL}" -q -c "
  INSERT INTO job_capabilities
    (capability_id, job_id, workspace_id, allowed_operations_json,
     max_llm_calls, max_tool_runs, max_item_creations, expires_at, created_at)
  VALUES
    ('cap-${JOB_ID}', '${JOB_ID}', '${WS_ID}',
     '[\"JOB_OPERATION_READ_DOCUMENT\",\"JOB_OPERATION_INVOKE_LLM\",\"JOB_OPERATION_CREATE_ITEM\",\"JOB_OPERATION_EMIT_EVAL\"]',
     50, 100, 50, NOW() + INTERVAL '1 hour', NOW());
"

echo ""
echo "── Job 作成 ─────────────────────────────────────────"
printf "  job_id : %s\n" "${JOB_ID}"
printf "  tree_id: %s\n" "${TREE_ID}"

# ── GenerateExecutionPlan ───────────────────────────────────────────────────
echo ""
echo "── GenerateExecutionPlan ────────────────────────────"
PLAN_HTTP_FILE=$(mktemp)
PLAN_BODY=$(curl -s -o "${PLAN_HTTP_FILE}" -w "%{http_code}" \
  -H "Content-Type: application/json" \
  -H "X-Worker-Token: ${WORKER_TOKEN}" \
  "${WORKER_URL}/synthify.tree.v1.WorkerService/GenerateExecutionPlan" \
  -d "{
    \"job_id\":       \"${JOB_ID}\",
    \"job_type\":     \"JOB_TYPE_PROCESS_DOCUMENT\",
    \"document_id\":  \"${DOC_ID}\",
    \"workspace_id\": \"${WS_ID}\",
    \"tree_id\":      \"${TREE_ID}\"
  }")
PLAN_STATUS="${PLAN_BODY}"
PLAN_RESP=$(cat "${PLAN_HTTP_FILE}")
rm -f "${PLAN_HTTP_FILE}"
echo "${PLAN_RESP}" | python3 -m json.tool 2>/dev/null || echo "${PLAN_RESP}"
if [[ "${PLAN_STATUS}" != "200" ]]; then
  echo "ERROR: GenerateExecutionPlan failed (HTTP ${PLAN_STATUS})"
  exit 1
fi

# ── ExecuteApprovedPlan ─────────────────────────────────────────────────────
echo ""
echo "── ExecuteApprovedPlan ──────────────────────────────"
echo "  GCS からファイル取得 → chunk → embed → synthesize → persist"
echo "  (embedding 生成を含むため数十秒かかります)"
EXEC_HTTP_FILE=$(mktemp)
EXEC_STATUS=$(curl -s -o "${EXEC_HTTP_FILE}" -w "%{http_code}" \
  -H "Content-Type: application/json" \
  -H "X-Worker-Token: ${WORKER_TOKEN}" \
  "${WORKER_URL}/synthify.tree.v1.WorkerService/ExecuteApprovedPlan" \
  -d "{
    \"job_id\":       \"${JOB_ID}\",
    \"job_type\":     \"JOB_TYPE_PROCESS_DOCUMENT\",
    \"document_id\":  \"${DOC_ID}\",
    \"workspace_id\": \"${WS_ID}\",
    \"tree_id\":      \"${TREE_ID}\",
    \"file_uri\":     \"${FILE_URI_INTERNAL}\",
    \"filename\":     \"${FILENAME}\",
    \"mime_type\":    \"text/markdown\"
  }")
EXEC_RESP=$(cat "${EXEC_HTTP_FILE}")
rm -f "${EXEC_HTTP_FILE}"
echo "${EXEC_RESP}" | python3 -m json.tool 2>/dev/null || echo "${EXEC_RESP}"
if [[ "${EXEC_STATUS}" != "200" ]]; then
  echo "ERROR: ExecuteApprovedPlan failed (HTTP ${EXEC_STATUS})"
  echo "  (worker logs: docker compose logs worker --tail=20)"
  exit 1
fi

# ── after スナップショット ────────────────────────────────────────────────────
echo ""
echo "── After ───────────────────────────────────────────"
AFTER_CHUNKS=$(${PSQL} -c "SELECT COUNT(*) FROM document_chunks WHERE document_id = '${DOC_ID}';" | tr -d ' ')
AFTER_EMBEDDED=$(${PSQL} -c "SELECT COUNT(*) FROM document_chunks WHERE document_id = '${DOC_ID}' AND embedding IS NOT NULL;" | tr -d ' ')
AFTER_ITEMS=$(${PSQL} -c "SELECT COUNT(*) FROM tree_items WHERE workspace_id = '${WS_ID}' AND created_by = 'llm_worker';" | tr -d ' ')

printf "  chunks    : %s → %s\n" "${BEFORE_CHUNKS}" "${AFTER_CHUNKS}"
printf "  embedded  : %s → %s\n" "${BEFORE_EMBEDDED}" "${AFTER_EMBEDDED}"
printf "  tree_items: %s → %s\n" "${BEFORE_ITEMS}" "${AFTER_ITEMS}"

# ── 生成された tree_items ────────────────────────────────────────────────────
echo ""
echo "── 生成された tree_items ────────────────────────────"
psql "${PG_URL}" --pset=pager=off --pset=columns=200 -c "
  SELECT
    REPEAT('  ', level) || label AS label,
    level,
    LEFT(description, 80) AS description
  FROM tree_items
  WHERE workspace_id = '${WS_ID}' AND created_by = 'llm_worker'
  ORDER BY created_at, level;
"

# ── chunk と embedding の確認 ────────────────────────────────────────────────
echo ""
echo "── chunks & embedding ───────────────────────────────"
psql "${PG_URL}" --pset=pager=off --pset=columns=200 -c "
  SELECT
    chunk_id,
    heading,
    LEFT(text, 60) || '...' AS text_preview,
    CASE WHEN embedding IS NOT NULL THEN vector_dims(embedding)::text ELSE 'none' END AS dims
  FROM document_chunks
  WHERE document_id = '${DOC_ID}'
  ORDER BY chunk_id;
"

# ── job ステータス ────────────────────────────────────────────────────────────
echo ""
echo "── Job ステータス ──────────────────────────────────"
psql "${PG_URL}" --pset=pager=off --pset=columns=200 -c "
  SELECT job_id, status, current_stage, error_message
  FROM document_processing_jobs
  WHERE job_id = '${JOB_ID}';
"

echo ""
echo "══════════════════════════════════════════════════════"
echo "  他のドキュメントを試す:"
echo "    bash scripts/worker_demo.sh doc_llm_2   # 日本語"
echo "    bash scripts/worker_demo.sh doc_llm_3   # レガシー移行メモ"
echo "══════════════════════════════════════════════════════"
