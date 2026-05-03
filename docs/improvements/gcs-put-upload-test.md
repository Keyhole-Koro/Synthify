# GCS PUT Upload Test

GCS への直接アップロード（署名付き URL を使用した PUT リクエスト）の信頼性を担保するためのテスト。

## 現状
- `scripts/seed_gcs.sh` では `fake-gcs-server` の `POST` アップロード API を直接叩いている。
- ブラウザ（Frontend）からは `GCS_UPLOAD_URL_BASE` を使用して `PUT` リクエストを送る想定だが、この経路の自動テストが不足している。
- `fake-gcs-server` と本番の `Cloud Storage` で URL 形式や挙動が微妙に異なる可能性がある（特に `%2F` の扱いなど）。

## 課題
- Frontend が発行した PUT URL に対して、実際にファイルをアップロードできるかの結合テストがない。
- `Content-Type` や `x-goog-meta-*` ヘッダーが正しく反映されるかの検証が必要。
- 巨大なファイルのマルチパートアップロード（必要であれば）の検証。

## 改善案
- `api` 層の統合テストに、実際に `fake-gcs-server` に対して PUT を発行するテストケースを追加する。
- 以下のシナリオをカバーする：
  1. API から PUT URL を取得
  2. 取得した URL に対して `curl` または `http.Client` で `PUT` リクエストを送信
  3. アップロードされたファイルが `INTERNAL_GCS_UPLOAD_URL_BASE` 経由で正しく読み取れるか確認
  4. 異常系：期限切れ URL、不正な Workspace ID でのアップロード試行などが適切に拒否されるか（本番相当のシミュレーション）
