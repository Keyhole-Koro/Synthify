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
| `GCS_UPLOAD_URL_BASE` | クライアント（ブラウザ）向けの PUT upload URL ベース | `http://localhost:4443/storage/v1/b/synthify-uploads/o` | `https://storage.googleapis.com/upload/storage/v1/b/synthify-uploads/o` |
| `INTERNAL_GCS_UPLOAD_URL_BASE` | Worker がファイルを **取得** するときのベース（コンテナ内部） | `http://gcs:4443/storage/v1/b/synthify-uploads/o` | `https://storage.googleapis.com/storage/v1/b/synthify-uploads/o` |

## URL の組み立て

### ファイル取得 URL（Worker が `sourcefiles.Fetch` で使う）

```
{INTERNAL_GCS_UPLOAD_URL_BASE}/{workspace_id}%2F{document_id}?alt=media
```

例:
```
http://gcs:4443/storage/v1/b/synthify-uploads/o/ws_seed_1%2Fdoc_llm_1?alt=media
```

- `%2F` は `/` のエンコード。オブジェクト名にスラッシュを含む場合、GCS JSON API はパス区切りと区別するためエンコードが必要
- `?alt=media` でオブジェクトのメタデータではなく本体を返す

生成コード: [shared/app/bootstrap.go](../shared/app/bootstrap.go) `PublicUploadURLGenerator`

```go
fmt.Sprintf("%s/%s%%2F%s?alt=media", base, workspaceID, documentID)
```

### ファイルアップロード URL（クライアントが PUT で使う）

```
{GCS_UPLOAD_URL_BASE}/{workspace_id}%2F{document_id}?alt=media
```

例（ローカル）:
```
http://localhost:4443/storage/v1/b/synthify-uploads/o/ws_seed_1%2Fdoc_llm_1?alt=media
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
  - gcs:4443          # コンテナ間通信のホスト名
  - -external-url
  - http://localhost:4443  # ホストマシンからのアクセス用（フロントエンド向け）
```

- `-public-host` はコンテナ内部からのアクセスに使うホスト名
- `-external-url` はブラウザからアクセスするときのベース URL（presigned URL 等に埋め込まれる）

## 本番（Cloud Run）との違い

| | ローカル | 本番 |
|--|---------|------|
| ストレージ | fake-gcs-server | Cloud Storage |
| 認証 | なし | ADC（Workload Identity） |
| アップロード | PUT to JSON API | Signed URL（PUT） |
| Worker 取得 | JSON API `?alt=media` | 同左（ADC で認証） |
| バケット名 | `synthify-uploads`（固定） | 環境ごとに異なる |
