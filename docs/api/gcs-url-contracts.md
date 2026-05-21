# GCS URL Contracts

fake-gcs-server（開発）と Cloud Storage（本番）で使う URL パターンの仕様。

## オブジェクトパス規約

```
{workspace_id}/{document_id}
```

例: `ws_seed_1/doc_llm_1`

## 環境変数

| 変数 | 用途 | ローカルデフォルト | 本番例 |
|------|------|-------------------|--------|
| `GCS_UPLOAD_URL_BASE` | クライアント（ブラウザ）向け upload API のベース | `http://127.0.0.1:4443` | `https://storage.googleapis.com` |
| `INTERNAL_GCS_UPLOAD_URL_BASE` | Worker がファイルを **取得** するときの API ベース | `http://127.0.0.1:4443` | `https://storage.googleapis.com` |

## URL の組み立て

`BuildDocumentUploadURL` と `BuildDocumentSourceURL` は同じ document を指すが、用途が違うためエンドポイントを分けている。

- `BuildDocumentUploadURL`: ブラウザがファイル本体を GCS に `POST` するための書き込み用 URL を作る
- `BuildDocumentSourceURL`: worker が保存済みファイルを GCS から `GET` するための読み取り用 URL を作る

違いは主に 2 つ:

- upload 用は `/upload/storage/v1/...` と `?uploadType=media&name=...` を使う
- source 用は `/storage/v1/...` と `?alt=media` を使う

### ファイル取得 URL（Worker が `sourcefiles.Fetch` で使う）

```
{INTERNAL_GCS_UPLOAD_URL_BASE}/storage/v1/b/synthify-uploads/o/{workspace_id}%2F{document_id}?alt=media
```

例:
```
http://127.0.0.1:4443/storage/v1/b/synthify-uploads/o/ws_seed_1%2Fdoc_llm_1?alt=media
```

- `%2F` は `/` のエンコード。オブジェクト名にスラッシュを含む場合、GCS JSON API はパス区切りと区別するためエンコードが必要
- `?alt=media` でオブジェクトのメタデータではなく本体を返す

生成コード: [gcsurls.go](/home/unix/Synthify/internal/platform/storage/gcsurls.go) `BuildDocumentSourceURL`

```go
BuildDocumentSourceURL(baseURL, workspaceID, documentID)
```

`GCS_UPLOAD_URL_BASE` / `INTERNAL_GCS_UPLOAD_URL_BASE` は path を含まない origin のみを許可する。
例: `http://127.0.0.1:4443`

### ファイルアップロード URL（クライアントが POST で使う）

```
{GCS_UPLOAD_URL_BASE}/upload/storage/v1/b/synthify-uploads/o?uploadType=media&name={workspace_id}/{document_id}
```

例（ローカル）:
```
http://127.0.0.1:4443/upload/storage/v1/b/synthify-uploads/o?uploadType=media&name=ws_seed_1/doc_llm_1
```

### seed スクリプトのアップロード（multipart upload API）

`scripts/seed_gcs.sh` は fake-gcs の upload API を直接叩く:

```
POST {GCS_URL}/upload/storage/v1/b/{bucket}/o?uploadType=media&name={workspace_id}/{document_id}
```

こちらはオブジェクト名をクエリパラメータで渡すのでエンコード不要。

## fake-gcs-server の設定（compose.yaml）

```yaml
command:
  - -scheme
  - http
  - -port
  - "4443"
  - -public-host
  - localhost:4443
  - -external-url
  - http://localhost:4443  # ホストマシンからのアクセス用（フロントエンド向け）
```

- `host network` 構成では `backend` / `worker` からも `127.0.0.1:4443` を使う
- `-external-url` はブラウザからアクセスするときのベース URL

## 本番（Cloud Run）との違い

| | ローカル | 本番 |
|--|---------|------|
| ストレージ | fake-gcs-server | Cloud Storage |
| 認証 | なし | ADC（Workload Identity） |
| アップロード | PUT to JSON API | Signed URL（PUT） |
| Worker 取得 | JSON API `?alt=media` | 同左（ADC で認証） |
| バケット名 | `synthify-uploads`（固定） | 環境ごとに異なる |
