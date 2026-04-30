#!/usr/bin/env bash
# seed_gcs.sh — demo_docs/ のファイルを fake-gcs-server にアップロードする
#
# 前提: docker compose up -d gcs
#
# 使い方:
#   bash scripts/seed_gcs.sh

set -euo pipefail

GCS_URL="${GCS_URL:-http://localhost:4443}"
BUCKET="synthify-uploads"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="${SCRIPT_DIR}/demo_docs"

# バケットが存在しない場合は作成
curl -sf -X POST "${GCS_URL}/storage/v1/b?project=synthify-local" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"${BUCKET}\"}" > /dev/null 2>&1 || true

echo "Uploading demo documents to fake-gcs (${GCS_URL}/storage/v1/b/${BUCKET})..."
echo ""

upload() {
  local ws_id="$1"
  local doc_id="$2"
  local filepath="$3"
  local filename
  filename="$(basename "${filepath}")"
  local object_path="${ws_id}/${doc_id}"

  curl -sf -X POST \
    "${GCS_URL}/upload/storage/v1/b/${BUCKET}/o?uploadType=media&name=${object_path}" \
    -H "Content-Type: text/markdown" \
    --data-binary "@${filepath}" > /dev/null

  echo "  ✓ ${filename} → gs://${BUCKET}/${object_path}"
}

upload "ws_seed_1" "doc_llm_1" "${DOCS_DIR}/microservices_design.md"
upload "ws_seed_2" "doc_llm_2" "${DOCS_DIR}/clinical_trial_protocol.md"
upload "ws_seed_1" "doc_llm_3" "${DOCS_DIR}/legacy_migration_notes.md"

echo ""
echo "Done. Worker は GCS_UPLOAD_URL_BASE/{ws_id}/{doc_id} でファイルを取得します。"
echo "次: bash scripts/worker_demo.sh doc_llm_1"
