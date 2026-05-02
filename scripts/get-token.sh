#!/usr/bin/env bash
# get-token.sh — Firebase Auth emulator からIDトークンを取得する
#
# Usage:
#   TOKEN=$(bash scripts/get-token.sh)
#   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/...
#
# オプション:
#   E2E_EMAIL    サインインするメールアドレス (default: e2e@test.local)
#   E2E_PASSWORD パスワード (default: e2e-password)
#   AUTH_URL     Firebase Auth emulator URL (default: http://localhost:9099)

set -euo pipefail

EMAIL="${E2E_EMAIL:-e2e@test.local}"
PASSWORD="${E2E_PASSWORD:-e2e-password}"
AUTH_URL="${AUTH_URL:-http://localhost:9099}"
SIGN_UP_URL="${AUTH_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake"
SIGN_IN_URL="${AUTH_URL}/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake"

# ユーザーを作成（既存の場合は 400 を無視）
curl -s -o /dev/null -X POST "$SIGN_UP_URL" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"returnSecureToken\":true}" || true

# サインインしてIDトークンを取得
TOKEN=$(curl -s -X POST "$SIGN_IN_URL" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"returnSecureToken\":true}" \
  | jq -r '.idToken')

if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "ERROR: Failed to get token" >&2
  exit 1
fi

echo "$TOKEN"
