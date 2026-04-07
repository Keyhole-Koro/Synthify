# 05. API Specification

## RPC Contract

### Overview

- フロントエンドとバックエンド間の同期通信は `Connect RPC` を使用する
- 契約定義は `Protocol Buffers` で管理する
- 外部公開 REST API は初期スコープに含めない
- 重い処理は RPC からジョブを起動し、完了確認は status 取得 RPC で行う
- proto の叩き台は [proto/README.md](proto/README.md) を参照する

### ID Conventions

- 初期実装では時系列ソートしやすい文字列 ID として `ULID` を採用する
- `workspace_id` は `ws_<ULID>`、`document_id` は `doc_<ULID>` を使う
- `node_id` は `nd_<ULID>`、`canonical_node_id` は `cn_<ULID>` を使う

## Services

### DocumentService

#### CreateDocument

document を作成し、ファイル upload 用の署名付き URL を発行する。

#### Request

- `workspace_id`
- `filename`
- `mime_type`
- `file_size`

#### Response

```json
{
  "document": {
    "document_id": "doc_01JQ8Y5M4R7C1N5T8V2K6P9L3B",
    "status": "uploaded"
  },
  "upload_url": "https://storage.googleapis.com/...",
  "upload_method": "PUT",
  "upload_content_type": "application/pdf"
}
```

#### Notes

- 実ファイル転送は `CreateDocument` のレスポンスで返した署名付き URL に対してクライアントが直接実行する
- upload 完了後に `StartProcessing` を呼び出して解析を開始する

#### GetUploadURL

GCS 署名付き PUT URL を発行する。クライアントはこの URL に対して直接ファイルをアップロードし、完了後に `CreateDocument` を呼ぶ。

#### Request Parameters

- `workspace_id`
- `filename`
- `mime_type`
- `file_size`

#### Response Example

```json
{
  "upload_url": "https://storage.googleapis.com/...",
  "upload_token": "tok_...",
  "expires_at": "2026-03-29T12:00:00Z"
}
```

#### GetDocument

document のメタデータと処理状態を取得する。

#### ListDocuments

document 一覧と処理状態を取得する。`workspace_id` でフィルタする。

#### StartProcessing

document の解析を開始する。

#### Preconditions

- 対象 document の実ファイル upload が完了していること
- `documents.status` が `uploaded` であること
- upload 未完了または `processing` / `completed` 状態の document に対してはエラーを返す
- `force_reprocess=true` の場合のみ `completed` または `failed` の document を再処理対象として受け付ける

#### Response

```json
{
  "document_id": "doc_01JQ8Y5M4R7C1N5T8V2K6P9L3B",
  "status": "processing",
  "job_id": "job_001"
}
```

#### ResumeProcessing

`pending_normalization` 状態の document の処理を再開する。

正規化ツールが承認された際に自動的に呼び出されるが、自動再開が失敗した場合や手動で再開したい場合に `dev` / `editor` ロールから明示的に呼び出す。

#### Preconditions

- `documents.status` が `pending_normalization` であること
- それ以外の状態では `FAILED_PRECONDITION` を返す

#### Response

```json
{
  "document_id": "doc_01JQ8Y5M4R7C1N5T8V2K6P9L3B",
  "status": "processing",
  "job_id": "job_002"
}
```

### WorkspaceService

workspace の作成・参照・メンバー管理を担う。

#### CreateWorkspace

新しい workspace を作成する。作成者は自動的に `owner` として `workspace_members` に登録される。

#### GetWorkspace

workspace のメタデータとメンバー一覧を取得する。workspace に所属するメンバーのみ呼び出し可能。

#### ListWorkspaces

自分が所属している workspace の一覧を返す。

#### InviteMember

メールアドレスを指定してメンバーを招待する。`editor` / `owner` のみ呼び出し可能。

#### Request Parameters

- `workspace_id`
- `email` : 招待するユーザーのメールアドレス
- `role` : `editor` / `viewer`
- `is_dev` : `/dev/stats` アクセス権（デフォルト: `false`）

#### Notes

- 対象メールアドレスが既存 Firebase Auth ユーザーの場合は即座に `workspace_members` に追加する
- 未登録の場合は `pending invite` として workspace に記録し、当該メールアドレスで初回ログインした時点で membership を確定する。メール送信自体は初期スコープ外。
- `owner` ロールは招待では付与できない（`TransferOwnership` RPC で別途対応）
- プランの `max_members` を超える招待はエラーを返す

#### UpdateMemberRole

メンバーのロールまたは `is_dev` フラグを変更する。`owner` のみ呼び出し可能。

#### Notes

- `owner` 自身のロールは変更不可（`TransferOwnership` を使う）

#### RemoveMember

メンバーを workspace から削除する。`owner` のみ呼び出し可能。`owner` 自身は削除不可。

#### TransferOwnership

workspace の ownership を別メンバーへ移譲する。`owner` のみ呼び出し可能。

#### Preconditions

- `new_owner_user_id` が同一 workspace の既存メンバーであること
- 現 owner の `is_dev=true` は維持してよいが、base role は `editor` へ降格する
- 新 owner は `role=owner` に更新される

### GraphService

#### GetGraph

ペーパーツリー構築用のノード集合を取得する。`BigQuery` を正本として返す。フロントはこのレスポンスから `PaperMap` を構築する。ツリー構造と関連先リンクは各 node の `summary_html` とアプリ側の parent/child 解決ロジックで扱う。

#### Request Parameters

- `document_id`
- `workspace_id`
- `category_filters`
- `level_filters`
- `limit`
- `source_filename` : zip 内の特定ファイル由来のノードに絞り込む（省略時は全ファイル対象）
- `resolve_aliases` : canonical ノードへ集約して返すか

#### Response Example

```json
{
  "document_id": "doc_01JQ8Y5M4R7C1N5T8V2K6P9L3B",
  "nodes": [
    {
      "id": "nd_01JQ8Y7M6Y7YJ8V0X3D4K9P2AB",
      "canonical_node_id": "cn_01JQ8YCH9R2V6M4B8T1K5N7PQS",
      "scope": "document",
      "label": "販売戦略",
      "level": 1,
      "category": "concept",
      "description": "販売拡大のための上位方針",
      "summary_html": "<p>...</p>"
    }
  ]
}
```

#### FindPaths

2 ノード間の多段経路を検索する。`Spanner Graph` を使い、複数経路候補を返す。

この仕様では、各 path に付く根拠導線を `PathEvidenceRef` と呼ぶ。`PathEvidenceRef` は軽量参照であり、詳細本文は返さない。

#### Request Parameters

- `source_node_id`
- `target_node_id`
- `workspace_id`
- `max_depth`
- `limit`
- `cross_document`
- `document_ids`

#### Response Example

```json
{
  "graph": {
    "workspace_id": "ws_01JQ8Y2C9R4V6M1B8T5K7N3PQS",
    "document_id": "",
    "cross_document": true,
    "nodes": [
      {
        "id": "cn_01JQ8YCH9R2V6M4B8T1K5N7PQS",
        "document_id": "",
        "canonical_node_id": "cn_01JQ8YCH9R2V6M4B8T1K5N7PQS",
        "scope": "canonical",
        "label": "Sales Strategy",
        "level": 1,
        "category": "concept"
      },
      {
        "id": "cn_01JQ8YG6B1N4T8M3R7V2K5P9DX",
        "document_id": "",
        "canonical_node_id": "cn_01JQ8YG6B1N4T8M3R7V2K5P9DX",
        "scope": "canonical",
        "label": "SNS Campaign",
        "level": 2,
        "category": "concept"
      },
      {
        "id": "cn_01JQ8YJ2F4C6M9T1R3V8K5N7QW",
        "document_id": "",
        "canonical_node_id": "cn_01JQ8YJ2F4C6M9T1R3V8K5N7QW",
        "scope": "canonical",
        "label": "CV Rate 3.2%",
        "level": 3,
        "category": "entity",
        "entity_type": "metric"
      }
    ]
  },
  "paths": [
    {
      "node_ids": ["cn_01JQ8YCH9R2V6M4B8T1K5N7PQS", "cn_01JQ8YG6B1N4T8M3R7V2K5P9DX", "cn_01JQ8YJ2F4C6M9T1R3V8K5N7QW"],
      "hop_count": 2,
      "evidence_ref": {
        "source_document_ids": ["doc_01JQ8Y5M4R7C1N5T8V2K6P9L3B", "doc_01JQ8Y6B1N4T8M3R7V2K5P9DX"]
      }
    }
  ]
}
```

#### Notes On This Example

- `FindPathsResponse.graph` follows the current `Graph` message shape and therefore includes `workspace_id`, `document_id`, and `cross_document`
- In canonical scope, `id` and `canonical_node_id` are the same value by design
- `document_id` is empty in this example because it represents a cross-document canonical graph rather than a single document projection

#### Notes

- `GetGraph` は `BigQuery` を正本として返す。フロントはこのレスポンスで `paperMap` を構築する
- `FindPaths` は `Spanner Graph` を参照し、低レイテンシを優先する
- `FindPaths` は edge を返さず、`Spanner Graph` 上の node-to-node adjacency を辿って `node_ids` の列だけを返す
- adjacency は canonical node 間の近接関係として管理し、tree 上の親子関係・summary 内リンク・canonical クラスタ近傍を統合した探索用インデックスとして扱う
- `BigQuery` と `Spanner Graph` に同期遅延がある場合、`FindPaths` の結果は最新の抽出完了直後とわずかにずれる可能性がある
- 探索系 RPC は必ず `workspace_id` を受け取り、workspace 境界をまたがる探索は許可しない
- `cross_document=false` の場合は現在の document または `document_ids` の範囲だけを探索対象とする
- `cross_document=true` の場合は同一 workspace 内の canonical graph を探索対象とする
- `GraphPath.evidence_ref` は `PathEvidenceRef` を返し、path の根拠へ降りるための軽量参照のみを含む
- `PathEvidenceRef.source_document_ids` は path 根拠が見つかった document 群を返す
- `Node.scope=document` の場合 `id` は `nd_*` を返し、`canonical_node_id` は alias 解決済みなら補助属性として返す
- `Node.scope=canonical` の場合 `id` と `canonical_node_id` は同一の `cn_*` を返す

#### Node-only Path Model

- path の 1 hop は「exploration adjacency を持つ 2 canonical nodes の近接」を意味する
- exploration adjacency は永続 API 契約には露出しない内部概念とし、`Spanner Graph` 側だけで保持する
- adjacency 生成元の例は、親子配置、summary 内の `data-paper-id` 参照、canonical 化により近接した topic cluster である
- hop 数は `node_ids.length - 1` で計算する

### NodeService

#### GetGraphEntityDetail

詳細パネル表示用に、参照対象の詳細・根拠情報を取得する。document node / canonical node の違いは `target_ref` で表現し、バックエンド実装の切り替えを API 契約から分離する。

#### RecordNodeView

ユーザーがペーパーエクスプローラでノードを開いた（`OPEN_NODE`）際に呼び出し、閲覧履歴を `user_node_views` に記録する。`viewer` 以上が呼び出し可能。

バックエンドでデバウンスを行い、同一セッション内の連続 `OPEN_NODE` は 1 件としてカウントする。

#### CreateNode

ユーザーが手動でノードを追加する。`editor` / `owner` のみ呼び出し可能。

- `created_by` に呼び出しユーザーの `user_id` を記録する
- `parent_node_id` を指定した場合、親子関係をアプリケーション管理の tree 情報へ反映する
- AI 抽出パイプラインは経由しない（`status=completed` の document に追加する）

#### GetUserNodeActivity

ユーザーの閲覧ノード・追加ノードの一覧を取得する。workspace メンバー全員が任意メンバーのアクティビティを参照できる。

#### Request Parameters

- `workspace_id`
- `user_id` : 省略時は自分自身
- `document_id` : 省略時は workspace 全体
- `limit` : `viewed_nodes` / `created_nodes` それぞれの上限（デフォルト 50）

#### Response

- `activity.viewed_nodes` : 直近閲覧ノード（`last_viewed_at` 降順）
- `activity.created_nodes` : 手動追加ノード（`created_at` 降順）

#### ApproveAlias

`node_aliases.merge_status=suggested` のエイリアス候補を承認する。`dev` ロールのみ呼び出し可能。

#### Request Parameters

- `workspace_id`
- `canonical_node_id` : 統合先 canonical node
- `alias_node_id` : 統合元 document node

#### Response

- `merge_status: "approved"`

#### Notes

- 承認後、`canonical_nodes` の `label` / `description` は代表ノードの情報で更新される
- 既に `approved` / `rejected` のエイリアスに対しては `FAILED_PRECONDITION` を返す

#### RejectAlias

`node_aliases.merge_status=suggested` のエイリアス候補を却下する。`dev` ロールのみ呼び出し可能。

#### Request Parameters

- `workspace_id`
- `canonical_node_id`
- `alias_node_id`

#### Response

- `merge_status: "rejected"`

#### Notes

- 却下後、当該 `alias_node_id` は独立した canonical node として扱われる

#### Request Parameters

- `target_ref.workspace_id`
- `target_ref.scope` : `document` / `canonical`
- `target_ref.id` : `nd_*` または `cn_*`
- `target_ref.document_id` : `scope=document` のときのみ必須
- `resolve_aliases` : alias ノード指定時に canonical ノードへ寄せて返すか

#### Response Shape

- `detail.ref` : 要求した参照対象
- `detail.node` : 表示主体の node
- `detail.evidence.source_chunks` : 表示根拠 chunk
- `detail.evidence.source_document_ids` : 根拠 document 一覧
- `detail.representative_nodes` : `RepresentativeNodeRef` の実体として返す代表 document node 群

#### Notes

- `target_ref.scope=document` の場合は `BigQuery` 側の document node を正本として詳細を組み立てる
- `target_ref.scope=canonical` の場合は `Spanner Graph` の canonical node を起点にしつつ、`node_aliases` と代表 document node から evidence を補完する
- `detail.representative_nodes` は `scope=document` では空配列を返す
- フロントは `detail.ref.scope` を見て UI を切り替えるが、呼び出し先 RPC は常に `GetGraphEntityDetail` のみとする

### JobService

#### GetJobStatus

処理ジョブの状態を取得する。

#### Response Example

```json
{
  "job_id": "job_001",
  "status": "running"
}
```

### ToolService

`is_dev=true` のメンバーのみアクセス可能。workspace とは無関係のシステムグローバルなツール管理 API。詳細は [normalization-tools.md](../design/normalization-tools.md) を参照。

#### GenerateNormalizationTool

問題パターンの説明やサンプルデータをもとに、LLM から Python 正規化スクリプト案を生成する。

#### SaveNormalizationTool

生成されたスクリプトを `problem_pattern` と manifest とともに保存する（`draft` 状態）。

#### ListNormalizationTools

正規化ツール一覧を取得する。`approval_status` でフィルタ可能。

#### UpdateNormalizationToolStatus

ツールの状態を遷移させる（`draft` → `reviewed` → `approved` / `deprecated`）。`approved` 状態のみ本番適用可能。

#### RunNormalizationTool

ツールをサンドボックスで dry-run または本実行する。`APPLY` モードは `approved` のツールのみ実行可能。

#### GetNormalizationToolRun

ツール実行結果、差分、ログ、出力物参照を取得する。

### MonitoringService

`is_dev=true` のメンバーのみアクセス可能。パイプラインの運用監視・評価メトリクス取得を担う。`GraphService` とは責務が異なるため独立したサービスとして分離している。

#### GetPipelineStats

ドキュメント処理パイプラインの集計統計を取得する。ステージ別レイテンシ・完了数・LLM コスト等を返す。`since` を省略した場合は過去30日を対象とする。
この RPC は `PipelineMetrics` を返す。

#### GetExtractionStats

ノード抽出統計を取得する。`document_id` を省略した場合は全ドキュメント集計を返す。
この RPC は `ExtractionMetrics` を返す。

#### GetEvaluationTrend

週次の精度・再現率・レベル別正解率の推移を取得する。`weeks` を省略した場合は直近8週分を返す。
この RPC は `EvaluationMetrics` を返す。

#### ListFailedDocuments

処理に失敗したドキュメントの一覧を取得する。`since` でフィルタ可能。
この RPC は `ErrorMetrics` のための失敗一覧を返す。

#### GetNormalizationStats

正規化ツールの使用状況・自動/手動承認・却下の集計を取得する。`since` を省略した場合は過去30日を対象とする。
この RPC は `NormalizationMetrics` を返す。

#### GetAliasStats

`node_aliases` のステータス別件数・平均類似スコア・canonical ノード数を取得する。`workspace_id` を省略した場合は全 workspace 集計を返す。
この RPC は `AliasMetrics` を返す。

### BillingService

Stripe を通じた課金セッションを管理する。`WorkspaceService` とは責務が異なるため独立したサービスとして分離している。

#### CreateCheckoutSession

Stripe Checkout セッションを作成し、支払いページへのリダイレクト URL を返す。

#### CreatePortalSession

Stripe Customer Portal セッションを作成し、プラン変更・請求管理ページへのリダイレクト URL を返す。

## Role × RPC Authorization Matrix

各 RPC を呼び出せるロールを以下のマトリクスで管理する。`is_dev=true` は `MonitoringService` / `ToolService` / alias 管理 RPC へのアクセスを追加する。

| Service | RPC | viewer | editor | owner | is_dev |
|---|---|---|---|---|---|
| WorkspaceService | CreateWorkspace | ✓ | ✓ | ✓ | — |
| WorkspaceService | GetWorkspace | ✓ | ✓ | ✓ | — |
| WorkspaceService | ListWorkspaces | ✓ | ✓ | ✓ | — |
| WorkspaceService | InviteMember | — | ✓ | ✓ | — |
| WorkspaceService | UpdateMemberRole | — | — | ✓ | — |
| WorkspaceService | RemoveMember | — | — | ✓ | — |
| WorkspaceService | TransferOwnership | — | — | ✓ | — |
| DocumentService | CreateDocument / GetUploadURL | — | ✓ | ✓ | — |
| DocumentService | GetDocument / ListDocuments | ✓ | ✓ | ✓ | — |
| DocumentService | StartProcessing | — | ✓ | ✓ | — |
| DocumentService | ResumeProcessing | — | ✓ | ✓ | — |
| GraphService | GetGraph | ✓ | ✓ | ✓ | — |
| GraphService | FindPaths | ✓ | ✓ | ✓ | — |
| NodeService | GetGraphEntityDetail | ✓ | ✓ | ✓ | — |
| NodeService | RecordNodeView | ✓ | ✓ | ✓ | — |
| NodeService | CreateNode | — | ✓ | ✓ | — |
| NodeService | GetUserNodeActivity | ✓ | ✓ | ✓ | — |
| NodeService | ApproveAlias / RejectAlias | — | — | — | ✓ |
| JobService | GetJobStatus | ✓ | ✓ | ✓ | — |
| BillingService | CreateCheckoutSession / CreatePortalSession | — | — | ✓ | — |
| ToolService | すべての RPC | — | — | — | ✓ |
| MonitoringService | すべての RPC | — | — | — | ✓ |

### 認可ルール

- `viewer` は読み取りと `RecordNodeView` のみ可能。グラフを変更する RPC は呼び出せない
- `editor` は自分が所属する workspace の document 管理・処理実行・新規メンバー招待が可能
- `owner` は `editor` の全権限に加え、メンバー管理・ロール変更・プラン管理が可能
- `is_dev` は role と独立して付与され、開発者向け統計・ツール管理・alias 管理へのアクセスを追加する
- middleware の `auth.go` で Firebase ID Token を検証し、`workspace_members` テーブルから role と `is_dev` を取得して各 RPC のアクセス制御を行う

## Proto Design Guidelines

- package は単一の `synthify.graph.v1` とし、versioning は package suffix で管理する
- `.proto` ファイルは service 単位で分割し、1ファイル1service を原則とする
- service は `UserService`, `WorkspaceService`, `BillingService`, `DocumentService`, `GraphService`, `NodeService`, `JobService`, `ToolService`, `MonitoringService` に分割する
- `GraphService` は `GetGraph` / `FindPaths` の RPC のみを持つ（`ExpandNeighbors` は初期スコープ外）
- 運用監視・評価統計系 RPC は `MonitoringService` に分離する
- 課金系 RPC は `BillingService` に分離する
- request と response は用途単位で明示的に分ける
- 各 message / enum は所属ドメインの proto ファイルが所有する（`common.proto` は複数ドメインをまたぐ共有型のみ保持する）
- 複数 service から参照される message は service 個別 proto に重複定義しない
- package を domain ごとに細分化するのは初期スコープ外とし、import と生成コードの複雑化を避ける
- front の `paper-in-paper` で `PaperMap` を構築しやすい field 名を採用する
- ノード分類は `level` / `category` / `entity_type` を正とし、旧2値分類は持ち込まない

### Proto File Ownership

- `common.proto`: 複数ドメインをまたぐ共有型 (`DocumentChunk`) のみ保持し、service は定義しない
- `graph_types.proto`: `Node`, `Graph` および関連 enum (`NodeCategory`, `GraphProjectionScope` 等) を保持し、service は定義しない
- `document.proto`: `Document` / `DocumentLifecycleState` / `ExtractionDepth` と `DocumentService` (upload URL 発行・メタデータ取得・処理開始) を扱う
- `graph.proto`: `GraphService` (`GetGraph`, `FindPaths` の 2 RPC) のみを扱う
- `node.proto`: `NodeService` (`GetGraphEntityDetail`, `RecordNodeView`, `CreateNode`, `GetUserNodeActivity`, alias 管理 RPC) を扱う
- `job.proto`: `Job` / `JobType` / `JobLifecycleState` と `JobService` (非同期ジョブ状態取得) を扱う
- `tool.proto`: `NormalizationTool` / `NormalizationToolRun` / 関連 enum と `ToolService` を扱う
- `monitoring.proto`: `MonitoringService` (`PipelineMetrics`, `ExtractionMetrics`, `EvaluationMetrics`, `ErrorMetrics`, `NormalizationMetrics`, `AliasMetrics`) を扱う
- `billing.proto`: `BillingService` (Stripe セッション管理) を扱う
- `user.proto`: `UserService` (認証後のユーザー同期) を扱う
- `workspace.proto`: `WorkspaceService` (workspace 管理・メンバー管理) を扱う

### Package Evolution Policy

- 後方互換を壊す変更は `synthify.graph.v2` を新設して行う
- `v1` では field 追加を許容し、field 削除・型変更・意味変更は禁止する
- `buf breaking` を導入した時点で `main` ブランチとの差分を自動検証する

## Transport Policy

- ブラウザからは `Connect` プロトコルを優先利用する
- 将来的な他クライアント連携に備え、`gRPC` および `gRPC-Web` 互換を維持できる構成を優先する
- 長時間処理は unary RPC で完結させず、job 起動と status 参照に分ける
- 探索系 RPC は 1 画面あたり少数回の集約呼び出しを前提とし、N+1 的な node 単位 fetch を避ける

## Prompt and Extraction Policy

### Prompt Requirements

- ノードの `level` と `category` を明示的に割り当てさせる
- `level` は常に 0〜3 の固定4段階で割り当て、文書ごとに段数を変えさせない
- 出典 chunk の参照を必須にする
- JSON Schema に厳密に従うよう要求する

### Post-Processing Requirements

- ラベルの正規化
- 重複ノードの統合
- 不正 JSON に対する JSON repair を 1 回だけ試行する
- JSON repair 後も不正な場合は Gemini 再試行を最大 2 回まで行う
- JSON repair は syntax error のみを対象とし、semantic error は補正しない
- semantic error は schema 必須項目欠落、enum 不正値、参照不整合、制約違反を含む
- 構造成立に必須な項目はフォールバックせず、要素破棄または再試行対象とする
- `description`, `summary_html`, `entity_type` など品質補助項目に限ってフォールバックを許容する
- 不十分な出力時の fail handling
- chunk 抽出の確定失敗は document 全体の失敗として扱う
