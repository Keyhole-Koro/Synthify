# 04. Data Model

## State Naming Policy

状態値は用途ごとに state family を分けて扱う。実装では文字列の再利用ではなく、family ごとの enum / type alias を定義する前提とする。

### State Families

| Family | 用途 | 主な値 |
| --- | --- | --- |
| `DocumentLifecycleState` | document 全体の処理ライフサイクル | `uploaded` / `pending_normalization` / `processing` / `completed` / `failed` |
| `JobLifecycleState` | 非同期ジョブ全体の実行状態 | `queued` / `running` / `succeeded` / `failed` |
| `PipelineStageState` | 個別ステージの実行結果 | `pending` / `running` / `succeeded` / `failed` / `skipped` |
| `NormalizationReviewState` | 正規化ツールの承認状態 | `draft` / `reviewed` / `approved` / `deprecated` |
| `AliasMergeState` | canonical 候補の統合レビュー状態 | `suggested` / `approved` / `rejected` |
| `GraphProjectionScope` | API 上の graph 投影対象 | `document` / `canonical` |
| `SyncJobState` | BigQuery から探索基盤への同期ジョブ状態 | `queued` / `running` / `completed` / `failed` |

### Naming Rules

- `status` は永続テーブル上の状態カラム名に使う
- `state` は仕様上の抽象概念として使い、必要に応じて family 名を付ける
- `scope` は `GraphProjectionScope` を指す専用語として使う
- `approved` や `failed` のような同名値は family が違えば意味も異なるため、仕様と実装の両方で混同しない

## Named Reference Objects

軽量な参照オブジェクトや中間成果物も名前を固定して扱う。

### Reference Families

| Name | 用途 | 中身 |
| --- | --- | --- |
| `PathEvidenceRef` | path から根拠へ降りるための軽量参照 | `source_document_ids` |
| `RepresentativeNodeRef` | canonical node に対応する代表 document node 参照 | representative document node の ID 群 |
| `DocumentBrief` | document 全体の高レイヤー要約 | 主題、level 0〜1 候補、主要 claim / entity、章構成 |
| `SectionBrief` | section 単位の高レイヤー要約 | 主題、代表ノード候補、接続ヒント、entity / metric 要約 |

### Reference Rules

- `Ref` は本文を持たない軽量参照を表す
- `Brief` は再生成可能な中間成果物を表す
- `PathEvidenceRef` は UI の導線用であり、source chunk 本文そのものは含めない

## Tables

### users

Firebase Auth でログインした際に初回のみ自動作成する。

| Column | Type | Description |
| --- | --- | --- |
| user_id | STRING | Firebase Auth UID |
| email | STRING | メールアドレス |
| display_name | STRING | 表示名 |
| created_at | TIMESTAMP | 初回ログイン日時 |
| last_login_at | TIMESTAMP | 最終ログイン日時 |

### user と workspace の関係

```
User
  │ 1
  │ ├─ 複数の workspace を作成できる（owner）
  │ │      workspaces.owner_id = user_id
  │ │
  │ └─ 複数の workspace にメンバーとして参加できる
  │        workspace_members.user_id = user_id
  │ *
Workspace
  │ 1
  └─ 複数の document を持つ
         documents.workspace_id = workspace_id
```

- 1ユーザーは複数の workspace を作成できる
- 1ユーザーは複数の workspace にメンバーとして参加できる
- 1つの workspace は複数の document を持つ
- workspace の作成者は自動で `owner` として `workspace_members` に登録される

### workspaces

`workspace_id` も name や作成順の連番から直接導出せず、永続化時にバックエンドがグローバル一意な値を採番する。初期実装では `ws_<ULID>` 形式を推奨する。

| Column | Type | Description |
| --- | --- | --- |
| workspace_id | STRING | ワークスペース識別子 |
| name | STRING | ワークスペース名 |
| owner_id | STRING | オーナーのユーザーID（Firebase Auth UID） |
| plan | STRING | `free` / `pro` |
| stripe_customer_id | STRING | Stripe の顧客ID |
| stripe_subscription_id | STRING | Stripe のサブスクリプションID |
| storage_used_bytes | INT64 | 使用済みストレージ容量 |
| created_at | TIMESTAMP | 作成日時 |
| updated_at | TIMESTAMP | 更新日時 |

### workspace_members

| Column | Type | Description |
| --- | --- | --- |
| workspace_id | STRING | ワークスペース識別子 |
| user_id | STRING | メンバーのユーザーID |
| role | STRING | `owner` / `editor` / `viewer` |
| is_dev | BOOL | `/dev/stats` アクセス権（role と独立して付与） |
| invited_at | TIMESTAMP | 招待日時 |
| invited_by | STRING | 招待したユーザーID |

#### Role Values

- `owner` : workspace の管理者。editor 権限をすべて持ち、加えてメンバーのロール変更・削除・workspace 削除・プラン変更が可能。workspace 作成者が自動的に付与される。1 workspace に 1 名のみ。
- `editor` : ドキュメントのアップロード・削除・処理実行・新規メンバーの招待が可能
- `viewer` : グラフの閲覧・ノードビュー記録のみ可能

#### is_dev Flag

- `true` の場合 `/dev/stats` にアクセスできる。`role` が `viewer` でも付与可能。
- workspace 作成者（owner）は初期値 `true`。招待時はデフォルト `false`。

### documents

`document_id` も表示名やアップロード順の連番から直接導出せず、永続化時にバックエンドがグローバル一意な値を採番する。初期実装では `doc_<ULID>` 形式を推奨する。

| Column | Type | Description |
| --- | --- | --- |
| document_id | STRING | ドキュメント識別子 |
| workspace_id | STRING | 所属ワークスペース識別子 |
| uploaded_by | STRING | アップロードしたユーザーID |
| filename | STRING | 元ファイル名 |
| gcs_uri | STRING | 保存先 URI |
| mime_type | STRING | MIME type |
| file_size | INT64 | ファイルサイズ |
| status | STRING | 処理状態 |
| extraction_depth | STRING | 抽出粒度（`full` / `summary`）。デフォルトは `full` |
| created_at | TIMESTAMP | 作成日時 |
| updated_at | TIMESTAMP | 更新日時 |

#### Status Values

- family 名: `DocumentLifecycleState`
- `uploaded` : メタデータ登録とファイル upload 完了後、解析開始前
- `pending_normalization` : 正規化ツールの承認待ちで処理を停止中
- `processing`
- `completed`
- `failed`

### document_chunks

| Column | Type | Description |
| --- | --- | --- |
| document_id | STRING | ドキュメント識別子 |
| chunk_id | STRING | chunk 識別子 |
| chunk_index | INT64 | chunk 順序 |
| text | STRING | chunk テキスト |
| source_filename | STRING | 元ファイル名（zip 内ファイルの場合は展開後のファイル名、単ファイルの場合は document の filename と同値） |
| source_page | INT64 | 元ページ番号 |
| source_offset_start | INT64 | 開始オフセット |
| source_offset_end | INT64 | 終了オフセット |

### nodes

詳細な抽出戦略は [extraction-strategy.md](../design/extraction-strategy.md) を参照。

`node_id` は表示ラベルや LLM が返す `local_id` から直接導出せず、永続化時にバックエンドがグローバル一意な値を採番する。初期実装では `nd_<ULID>` 形式を推奨する。

| Column | Type | Description |
| --- | --- | --- |
| document_id | STRING | ドキュメント識別子 |
| node_id | STRING | ノード識別子 |
| extraction_local_id | STRING | LLM 抽出時のローカル識別子（document 内で追跡用） |
| label | STRING | 表示ラベル |
| level | INT64 | 階層レベル（0=ドメイン / 1=概念 / 2=施策・アクション / 3=詳細） |
| category | STRING | ノードカテゴリ（`concept` / `entity` / `claim` / `evidence` / `counter`） |
| entity_type | STRING | エンティティ種別（category=entity のみ: `organization` / `person` / `metric` / `date` / `location`） |
| description | STRING | ノード説明 |
| summary_html | STRING | ノードサマリの HTML（構造タグのみ、CSS はアプリ側注入）。null の場合は description にフォールバック |
| source_chunk_id | STRING | 出典 chunk |
| confidence | FLOAT64 | 生成信頼度 |
| created_by | STRING | 手動追加したユーザーID（AI 抽出ノードは null） |
| created_at | TIMESTAMP | 作成日時 |

- `created_by` は null の場合 AI 抽出ノード、non-null の場合はユーザーが手動追加したノードを示す
- API 返却時の `canonical_node_id` は `node_aliases` を解決して補完する派生属性であり、`nodes` テーブルの永続カラムには含めない
- `GetGraph` の node は `id = node_id`, `scope = document` を返す
- `FindPaths` の node は `id = canonical_node_id`, `scope = canonical` を返す
- `scope` は `GraphProjectionScope` として扱う

#### Node Category Values

- `concept` : 抽象的・具体的な概念
- `entity` : 実体（組織・人物・数値・日付）
- `claim` : 主張・判断・結論
- `evidence` : 主張を支持する根拠・事例
- `counter` : 主張への反論・留意点

#### Node Level Values

- `0` : ドメイン（文書全体を覆う最上位概念）
- `1` : 概念（主要なテーマ・方針）
- `2` : 施策・アクション（具体的な取り組み）
- `3` : 詳細（数値・固有名詞・具体的事実）

### user_node_views

ユーザーがペーパーエクスプローラで開いた（OPEN_NODE した）ノードの閲覧履歴を記録する。

| Column | Type | Description |
| --- | --- | --- |
| workspace_id | STRING | ワークスペース識別子 |
| user_id | STRING | 閲覧したユーザーID |
| node_id | STRING | 閲覧したノード識別子（`nd_*`） |
| document_id | STRING | ノードが属する document 識別子 |
| first_viewed_at | TIMESTAMP | 最初に開いた日時 |
| last_viewed_at | TIMESTAMP | 最後に開いた日時 |
| view_count | INT64 | 累計閲覧回数 |

- `(workspace_id, user_id, node_id)` を複合主キーとして扱い、同一ノードの再閲覧は `last_viewed_at` と `view_count` を更新する（upsert）
- `RecordNodeView` RPC で記録し、バックエンドでデバウンス（同一セッション内の連続 OPEN_NODE は 1 件としてカウント）する
- BigQuery の MERGE 文で upsert する

### node_aliases

トピックのカノニカル化とオントロジー統合に利用する。詳細は [topic-mapping.md](topic-mapping.md) を参照。

`canonical_node_id` は文書内 `node_id` とは別の識別空間として扱う。初期実装では `cn_<ULID>` 形式を推奨する。

| Column | Type | Description |
| --- | --- | --- |
| workspace_id | STRING | 所属ワークスペース識別子 |
| canonical_node_id | STRING | 正規ノード識別子 |
| alias_node_id | STRING | 統合元ノード識別子 |
| alias_label | STRING | 表記揺れラベル |
| similarity_score | FLOAT64 | 類似スコア |
| merge_status | STRING | `suggested` / `approved` / `rejected` |
| created_at | TIMESTAMP | 作成日時 |

- `merge_status` は `AliasMergeState` として扱う

### canonical_nodes

探索用 graph と document 横断 UI の主キーとして使う canonical node の属性を保持する。`Spanner Graph` への同期元になる。

| Column | Type | Description |
| --- | --- | --- |
| workspace_id | STRING | 所属ワークスペース識別子 |
| canonical_node_id | STRING | 正規ノード識別子 |
| label | STRING | canonical 表示ラベル |
| category | STRING | 代表カテゴリ |
| level_hint | INT64 | 代表 level |
| description | STRING | canonical 説明 |
| representative_node_id | STRING | 代表元 node_id |
| updated_at | TIMESTAMP | 更新日時 |

- `canonical_nodes.canonical_node_id` は探索 API の `Node.id` として露出する
- document ノードと canonical ノードの対応は `node_aliases.alias_node_id -> canonical_node_id` で追跡する

### document_briefs

`brief_generation` ステージの中間成果物を保存する。再処理時に再生成されるため正本ではなく補助データとして扱う。

| Column | Type | Description |
| --- | --- | --- |
| document_id | STRING | ドキュメント識別子 |
| job_id | STRING | 生成時のジョブ識別子 |
| main_topic | STRING | 文書全体の主題 |
| level0_candidates | JSON | level 0 候補ノードのラベル一覧 |
| level1_candidates | JSON | level 1 候補ノードのラベル一覧 |
| key_claims | JSON | 主要 claim のラベル一覧 |
| key_entities | JSON | 主要 entity のラベル一覧 |
| chapter_structure | JSON | 章構成（見出し・順序） |
| created_at | TIMESTAMP | 作成日時 |

### section_briefs

`brief_generation` ステージで heading unit ごとに生成する中間成果物を保存する。

| Column | Type | Description |
| --- | --- | --- |
| document_id | STRING | ドキュメント識別子 |
| job_id | STRING | 生成時のジョブ識別子 |
| chunk_id | STRING | 対応する先頭 chunk |
| section_index | INT64 | セクション順序 |
| heading | STRING | セクション見出し |
| main_topic | STRING | セクション主題 |
| representative_node_candidates | JSON | 代表ノード候補ラベル一覧 |
| connection_hints | JSON | 隣接セクションとの接続ヒント |
| entity_summary | JSON | entity 要約 |
| metric_summary | JSON | metric 要約 |
| created_at | TIMESTAMP | 作成日時 |

### graph_sync_jobs

`BigQuery` 正本から `Spanner Graph` への同期状態を管理する。

| Column | Type | Description |
| --- | --- | --- |
| sync_job_id | STRING | 同期ジョブ識別子 |
| workspace_id | STRING | 対象ワークスペース |
| document_id | STRING | 対象 document（全体同期時は null 可） |
| status | STRING | `queued` / `running` / `completed` / `failed` |
| synced_node_count | INT64 | 同期ノード数 |
| started_at | TIMESTAMP | 開始日時 |
| completed_at | TIMESTAMP | 完了日時 |

- `graph_sync_jobs.status` は `SyncJobState` として扱う

### processing_jobs

非同期ジョブとステージ別実行結果の追跡に利用する。

| Column | Type | Description |
| --- | --- | --- |
| job_id | STRING | ジョブ識別子 |
| document_id | STRING | 対象 document |
| job_type | STRING | `process_document` / `reprocess_document` |
| job_status | STRING | `queued` / `running` / `succeeded` / `failed` |
| stage_name | STRING | `raw_intake` / `normalization` / `text_extraction` / `semantic_chunking` / `brief_generation` / `pass1_extraction` / `pass2_synthesis` / `html_summary_generation` / `persistence` |
| stage_state | STRING | `pending` / `running` / `succeeded` / `failed` / `skipped` |
| error_message | STRING | 失敗理由 |
| created_at | TIMESTAMP | 作成日時 |
| started_at | TIMESTAMP | 開始日時 |
| completed_at | TIMESTAMP | 完了日時 |

- `job_status` は `JobLifecycleState` として扱う
- `stage_state` は `PipelineStageState` として扱う

## Future Tables

### document_topic_mappings

ドキュメントとトピック（`category=concept` かつ `level in (0, 1)` の canonical ノード）の対応関係を保存する。詳細は [topic-mapping.md](topic-mapping.md) を参照。

| Column | Type | Description |
| --- | --- | --- |
| mapping_id | STRING | マッピング識別子 |
| document_id | STRING | ドキュメント識別子 |
| topic_node_id | STRING | トピックノード識別子 |
| confidence | FLOAT64 | 信頼スコア |
| reason | STRING | LLM による判定理由 |
| method | STRING | `keyword` / `embedding` / `llm` / `manual` |
| created_at | TIMESTAMP | 作成日時 |

### node_scores

グラフアルゴリズムの計算結果を保存する。詳細は [graph-algorithms.md](../design/graph-algorithms.md) を参照。

| Column | Type | Description |
| --- | --- | --- |
| node_id | STRING | ノード識別子 |
| algo_type | STRING | `pagerank` / `degree_centrality` / `betweenness_centrality` / `community_id` |
| score | FLOAT64 | スコア値 |
| metadata | JSON | アルゴリズム固有の追加情報 |
| computed_at | TIMESTAMP | 計算日時 |

### graph_snapshots

- 再処理前後の結果比較に利用する

### plans

プランごとの制限値を管理する設定テーブル。

| Column | Type | Description |
| --- | --- | --- |
| plan | STRING | `free` / `pro` |
| storage_quota_bytes | INT64 | ストレージ上限 |
| max_file_size_bytes | INT64 | 1ファイルあたりの上限サイズ |
| max_uploads_per_day | INT64 | 1日あたりのアップロード上限 |
| max_members | INT64 | workspace メンバー上限 |
| allowed_extraction_depths | STRING | 使用可能な extraction_depth（カンマ区切り） |

#### デフォルト値

| | free | pro |
| --- | --- | --- |
| storage_quota_bytes | 1GB | 50GB |
| max_file_size_bytes | 50MB | 500MB |
| max_uploads_per_day | 10 | 200 |
| max_members | 3 | 20 |
| allowed_extraction_depths | `summary` | `full,summary` |

### normalization_tools

| Column | Type | Description |
| --- | --- | --- |
| tool_id | STRING | ツール識別子 |
| name | STRING | ツール名 |
| version | STRING | バージョン |
| description | STRING | 説明 |
| problem_pattern | STRING | 対処する問題パターン（自動マッチング用） |
| approval_status | STRING | `draft` / `reviewed` / `approved` / `deprecated` |
| approved_by | STRING | `llm` / `human` |
| llm_review_score | FLOAT64 | LLM 自動レビューの信頼スコア（0〜1） |
| llm_review_reason | STRING | LLM の判定理由 |
| created_by | STRING | 作成者ユーザーID |
| created_at | TIMESTAMP | 作成日時 |
| updated_at | TIMESTAMP | 更新日時 |

- `approval_status` は `NormalizationReviewState` として扱う

### normalization_tool_runs

- ツールの dry-run、本実行、差分、失敗情報の記録に利用する
