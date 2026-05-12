# API Architecture & Authorization Contract

このドキュメントは、Synthify API の設計、特に認可（Authorization）とハンドラーの責任範囲に関する「契約（Contract）」を定義する。

## 1. 認可モデル (Authorization)

API は、操作対象のリソースが Workspace または Document に紐づくかによって、以下の順番でアクセス権を確認しなければならない。

### Workspace リソースの場合
`workspace_id` が直接わかるリソース（例：Tree Item の操作など）の場合：

1. リクエストから `workspace_id` を取得する。
2. ログイン中の `user_id` を用いて、DB で `workspace_id` へのアクセス権があるかを確認する。
3. 権限があれば処理を続行し、無ければ HTTP 403 / 404 を返す。

### Document 紐付けリソースの場合 (Jobなど)
`workspace_id` が直接わからず、`job_id` のみが渡される場合：

1. リクエストの `job_id` から `document_processing_jobs` テーブルを引き、対象のジョブ情報を取得する。
2. ジョブ情報から親の `document_id` または `workspace_id` を取得する。
3. 取得した `workspace_id` と `user_id` を用いてアクセス権を確認する。
4. 権限があれば取得済みのジョブ情報を後続処理に渡し、無ければエラーとする。

## 2. API Handler と Service の責任分界点

- **Handler (Controller) の役割:**
  - HTTP リクエストの受け取りとバリデーション。
  - **認可 (Authorization) の実行。**
  - Service への委譲と、戻り値の protobuf / JSON への変換 (`mappers` の使用)。
- **Service の役割:**
  - 認可済みの安全なデータを使った、純粋なビジネスロジックの実行。
  - Repository の呼び出し。
  - Service 内で再度 DB を引いて認可チェックを行わない（Handler で取得したモデルをそのまま受け取る）。

*※ 以前は `Service` が認可を行っていたが、GraphQL や ConnectRPC など複数の入り口ができた場合に認可の重複や漏れが発生するため、この境界線を設定した。*
