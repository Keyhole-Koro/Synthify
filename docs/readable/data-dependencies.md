# Data Dependencies

## 目的

このメモは、Synthify のデータ依存を読みやすく整理し、採用するデータモデルを明文化するためのもの。

対象:

- `workspace`, `user`, `document`, `node`, `edge` の関係
- 現行構造のどこを捨て、どこを残すか
- 今後の source of truth と realtime read model の分離

## 現行構造

現行 schema は概ね次の関係になっている。

```txt
user
  └─ workspace_members
       └─ workspace
            └─ documents
                 ├─ nodes
                 └─ edges
```

現行テーブルの主な役割:

- `workspaces`
  作業単位。plan, storage quota, storage used を持つ
- `workspace_members`
  workspace と user の対応
- `documents`
  workspace に属するアップロード済みドキュメント
- `nodes`
  document に属するグラフノード
- `edges`
  document に属するグラフエッジ
- `node_views`
  user が workspace 内で node を見た履歴
- `node_aliases`
  workspace ごとの alias 扱い

現行の依存方向:

```txt
workspace_members -> workspaces
documents         -> workspaces
nodes             -> documents
edges             -> documents
edges             -> nodes
node_views        -> workspaces
node_views        -> documents
node_views        -> nodes
node_aliases      -> workspaces
node_aliases      -> nodes
```

この構造では、`document` を消すと `node` / `edge` も消え、graph は document の従属物として扱われる。

## 採用する考え方

主語を分ける。

- `account`
  契約、課金、容量上限の主語
- `workspace`
  作業場所、権限境界、UI 上の container
- `document`
  ソース資料
- `graph`
  workspace が扱う知識構造
- `node` / `edge`
  graph の構成要素
- Firestore
  realtime UI state の read model

採用後の全体像:

```txt
account
  ├─ account_users
  └─ workspaces
       ├─ documents
       └─ graph
            ├─ nodes
            ├─ edges
            ├─ node_sources
            └─ edge_sources

firestore
  workspaces/{workspaceId}/presence/{userId}
```

## 仕様 1: `account` を導入し、`workspace` の責務を分離する

今後、課金、契約、容量上限は `workspace` ではなく `account` に持つ。
また、`workspace` は `user` に直接依存させず、`account` 配下の資産として扱う。

役割分担:

- `account`
  契約、課金、容量上限の主語
- `account_users`
  account に誰が属しているか
- `workspace`
  account 配下の作業場所

この構成により:

- 1 account 配下に複数 workspace を持てる
- 容量制限を account 合算で扱える
- 同じ account の中で graph asset を共有しやすくなる
- `workspace` を `user` 管理から独立させられる

### テーブル

`accounts`

- `account_id`
- `name`
- `plan`
- `storage_quota_bytes`
- `storage_used_bytes`
- `max_file_size_bytes`
- `max_uploads_per_5h`
- `max_uploads_per_1week`
- `created_at`

`account_users`

- `account_id`
- `user_id`
- `role`
- `joined_at`

`workspaces`

- `workspace_id`
- `account_id`
- `name`
- `created_at`

### アクセスの考え方

初期実装では `workspace_members` は持たない。

- `user` は `account_users` を通じて `account` に所属する
- `workspace` は `account` に属する
- account に所属している user は、その account 配下の全 workspace にアクセスできる

`ListWorkspaces` は `account_users` と `workspaces` を join して取得する。

例:

```sql
SELECT w.*
FROM workspaces w
JOIN account_users au ON au.account_id = w.account_id
WHERE au.user_id = $1
ORDER BY w.created_at DESC;
```

### `workspaces` から外すもの

以下は `workspace` の属性ではなく `account` の契約属性として持つ。

- `plan`
- `storage_quota_bytes`
- `storage_used_bytes`
- `max_file_size_bytes`
- `max_uploads_per_5h`
- `max_uploads_per_1week`

## 仕様 2: ストレージ上限は account 単位で数える

ストレージ上限は `account.storage_quota_bytes`、使用量は `account.storage_used_bytes` で管理する。

最小案では `documents.file_size` の総和を account で集計する。

扱い:

- document 作成前に `storage_used_bytes + file_size <= storage_quota_bytes` を確認
- 成功時に `storage_used_bytes += file_size`
- document 削除時に減算

将来的に graph asset や derived artifact にも容量が乗るなら、別の usage ledger に分離する。

## 仕様 3: `plan` はプラン種別、制限値は `account` の snapshot として持つ

`accounts.plan` は free / registered / pro / anonymous のようなプラン種別を表す。

実際の制限判定は `plan` から毎回導出するのではなく、`accounts` に保持された具体的な制限値を使う。

対象となる制限値の例:

- `storage_quota_bytes`
- `max_file_size_bytes`
- `max_uploads_per_5h`
- `max_uploads_per_1week`

この方針により:

- UI や課金状態の表示は `plan` で扱える
- 実際の enforcement は具体的な制限値だけ見ればよい
- 特例アカウントや移行時の上書きに対応しやすい
- プラン内容の変更が既存 account に即時波及しない

初期プランの例:

- `anonymous`
  アカウントなし相当。かなり厳しい制限
- `registered`
  アカウントあり。少し緩い制限
- `pro`
  サブスク。より緩い制限

匿名利用でも、内部的には `plan = anonymous` の account として扱う前提にする。

## 仕様 4: `node` / `edge` は `document` ではなく `graph` に属する

`node` と `edge` は document の従属物ではなく、workspace が持つ graph の構成要素として扱う。
初期実装では `1 workspace = 1 graph` とする。

### テーブル

`graphs`

- `graph_id`
- `workspace_id`
- `name`
- `created_at`
- `updated_at`

`nodes`

- `node_id`
- `graph_id`
- `label`
- `category`
- `entity_type`
- `description`
- `summary_html`
- `created_by`
- `created_at`

`edges`

- `edge_id`
- `graph_id`
- `source_node_id`
- `target_node_id`
- `edge_type`
- `description`
- `created_at`

依存:

```txt
workspace -> graph -> nodes
workspace -> graph -> edges
```

意味:

- `node` / `edge` は document ではなく graph に属する
- workspace は graph を1つ持つ
- document は graph の owner ではなく source として provenance から参照される

`graphs.workspace_id` には unique 制約を置く前提とする。

## 仕様 5: provenance は join table で持つ

`node` / `edge` を document から切る代わりに、source 情報は別テーブルで持つ。

### テーブル

`node_sources`

- `node_id`
- `document_id`
- `chunk_id` nullable
- `source_text` nullable
- `confidence` nullable

`edge_sources`

- `edge_id`
- `document_id`
- `chunk_id` nullable
- `source_text` nullable
- `confidence` nullable

一意性:

- `node_sources` は `(node_id, document_id, chunk_id)` を unique にする
- `edge_sources` は `(edge_id, document_id, chunk_id)` を unique にする

意味:

- 1 node が複数 document に支えられる
- 1 edge が複数根拠を持てる
- document は owner ではなく source として扱われる

### 配列ではなく join table を使う理由

`nodes.document_ids: string[]` のような配列は採用しない。

理由:

- `node` と `document` は many-to-many だから
- `document_id` ごとの追加情報を持てるから
- SQL の join, filter, aggregate が素直だから
- 参照整合性を保ちやすいから
- 将来 `chunk_id`, `confidence`, `source_kind` を足しやすいから

API response では必要に応じて `document_ids: string[]` に整形して返してよいが、永続化は正規化テーブルで持つ。

## 仕様 6: ID は ULID をアプリケーション側で付与する

`account_id`, `workspace_id`, `document_id`, `graph_id`, `node_id`, `edge_id` などの主要 ID は `ULID` を採用する。

方針:

- 永続化される主要エンティティの ID はアプリケーション側で生成する
- LLM は永続 ID を生成しない
- join table は単独 ID を持たず、複合 unique 制約を基本にする

この方針により:

- 一意性を LLM に依存しない
- 再実行時の不安定な ID 生成を避けられる
- DB 制約と整合性検証を保存層で扱える

`account_users` は `id` を持たず `(account_id, user_id)` を unique にしてよい。

`node_sources` / `edge_sources` も同様に、単独 `id` より複合 unique 制約を優先する。

## 仕様 7: LLM worker は候補生成だけを担当する

LLM worker を使う場合でも、責務は node / edge / source の候補生成までにとどめる。

役割分担:

- `LLM worker`
  抽出、分類、要約、node 候補生成、edge 候補生成、source 対応候補生成
- `ingestion service`
  `ULID` 付与、`temp_id` 解決、整合性検証、重複排除、DB 保存

LLM が返す識別子は永続 ID ではなく仮 ID とする。

例:

```json
{
  "nodes": [
    { "temp_id": "n1", "label": "OpenAI" },
    { "temp_id": "n2", "label": "ChatGPT" }
  ],
  "edges": [
    { "source_temp_id": "n1", "target_temp_id": "n2", "edge_type": "develops" }
  ]
}
```

保存時にアプリケーション側で `temp_id -> ULID` を解決し、永続化する。

## 仕様 8: realtime UI state は Firestore に持つ

`node_views` のような高頻度・短寿命・リアルタイム反映したいデータは Postgres ではなく Firestore に持つ。

### Firestore に置くもの

- 今どの user がどの node を見ているか
- workspace 内の presence
- focused node
- 現在開いている document
- cursor や selection のような短寿命 UI state

### Postgres に残すもの

- `account`
- `workspace`
- `document`
- `graph`
- `node`
- `edge`
- membership / ACL
- quota / billing

Firestore は realtime projection、Postgres は source of truth として分ける。

### Firestore 形

最初の実装は event log ではなく、ユーザーごとの現在状態を持つ。

```txt
workspaces/{workspaceId}/presence/{userId}
```

ドキュメント例:

- `userId`
- `displayName`
- `photoURL`
- `documentId`
- `focusedNodeId`
- `updatedAt`

これで:

- 今誰がいるか
- 誰がどの node を見ているか
- 一定時間更新がない user を offline 扱いする

を扱う。

offline 判定は `updatedAt` ベースのクライアント判定で行う。

初期実装:

- クライアントが一定間隔で `updatedAt` を更新する
- 表示側は `updatedAt` の時刻差で online / offline を判定する
- 古い presence の cleanup は初期実装では必須にしない

### 注意

- ACL の source of truth は Firestore に置かない
- workspace membership の判定は backend / Postgres 側で持つ
- Firestore rules でも workspace member のみ書けるように制限する
- 高頻度更新は debounce する

長期保存や分析が必要になったら、非同期で backend / BigQuery / Postgres 集計テーブルへ転送する。

## 仕様 9: document の処理状態は job テーブルに分離する

`documents.status`, `current_stage`, `error_message`, `extraction_depth` のような処理状態は、`documents` ではなく別の job テーブルに持つ。

`document` は source asset として薄く保ち、処理の進行状況や失敗理由は `document_processing_jobs` に寄せる。

### テーブル

`document_processing_jobs`

- `job_id`
- `document_id`
- `graph_id` nullable
- `job_type`
- `status`
- `current_stage`
- `error_message`
- `params_json`
- `created_at`
- `updated_at`

この構成により:

- 同じ document に対する再処理や再抽出の履歴を持てる
- `document` 自体の属性と処理状態を分離できる
- graph 生成処理と document 本体の責務を混ぜずに済む

## 採用後の構造イメージ

```txt
users

accounts
account_users

workspaces
  -> account_id

documents
  -> workspace_id

document_processing_jobs
  -> document_id
  -> graph_id nullable

graphs
  -> workspace_id

nodes
  -> graph_id

edges
  -> graph_id
  -> source_node_id
  -> target_node_id

node_sources
  -> node_id
  -> document_id

edge_sources
  -> edge_id
  -> document_id

firestore
  workspaces/{workspaceId}/presence/{userId}
```

## 要約

今は:

```txt
workspace -> document -> graph
```

今後は:

```txt
account -> workspace -> graph
account -> workspace -> document
document -> graph provenance
firestore -> workspace realtime state
```

へ移行する。

## 未決定事項

現時点で主要な設計判断は確定している。

今後は実装時に具体的な初期値や閾値を詰めればよい。
