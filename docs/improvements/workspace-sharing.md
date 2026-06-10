# Workspace Sharing

ワークスペースを他ユーザーと共有する機能の設計メモ。**メンバー招待型**(email でユーザーを招待し role 付与)と**公開リンク型**(token を知っていれば無認証で閲覧)の両方を提供する。

## 方針(確定事項)

- 共有の形: **メンバー招待 + 公開リンクの両方**
- 権限粒度: proto の `WorkspaceRole` をそのまま使い、**owner / editor / viewer** で読み取り・書き込みを分ける
  - viewer = 閲覧のみ / editor = ドキュメント追加・処理 / owner = 共有管理
- 課金・quota: **課金が生じる操作を実行した本人(招待メンバー)の account 負担**
  - **課金が生じる操作**(ドキュメント処理・LLM 呼び出し等)は**登録済みユーザーの招待メンバーのみ**実行可。未登録招待・公開リンクは**閲覧専用**で課金ゼロ
  - その操作のコストは **workspace 所有者ではなく、操作を実行した本人の account** に計上する
  - storage(アップロード)も同様に「アップロードした本人の account の quota」を消費させるか要検討(下記未決事項)

## 現状調査(2026-06-09 時点)

共有の土台は半分できている。

- **proto**: [contracts/connectrpc/synthify/app/v1/workspace.proto](../../contracts/connectrpc/synthify/app/v1/workspace.proto) に `InviteMember` / `UpdateMemberRole` / `RemoveMember` / `TransferOwnership` と `WorkspaceMember`・`WorkspaceRole`(OWNER/EDITOR/VIEWER) が**定義済み**
- **handler**: ただし上記 RPC は全て `Unimplemented` を返すだけ([apps/api/internal/handler/workspace.go](../../apps/api/internal/handler/workspace.go) `InviteMember` 以下)。コメントに "managed at account level"
- **認可の正本**: [db/queries/workspaces.sql](../../db/queries/workspaces.sql) の `IsWorkspaceAccessible` = `workspaces JOIN account_users`。つまり「**同じ account のユーザーは全 workspace にアクセス可**」というフラットモデル
- **重要**: この `IsWorkspaceAccessible` 1 関数が workspace / tree / item / document **全リソースの認可ゲート**になっている(`service/workspace.go` `service/tree.go` `service/item.go` `service/document.go` + `handler/authz.go`)。ここを差し替えれば共有が全体に波及する
- membership は account 単位(`account_users`)しか無く、**workspace 単位の membership / role テーブルが存在しない**
- **課金の紐付け**: ジョブには `requested_by`(実行ユーザー ID)が記録される([apps/api/internal/service/document.go](../../apps/api/internal/service/document.go) `startProcessingJob`)が、usage 計上は `usage_events` の `account_id` + `workspace_id` 軸([db/queries/billing.sql](../../db/queries/billing.sql))。現状コストは **workspace に紐づく account(= 所有者)** に乗るとみられ、`requested_by` の account には計上していない

### つまりギャップは 3 つ

1. workspace 単位の membership テーブルが無い(account 単位しかない)
2. `IsWorkspaceAccessible` が account しか見ていない
3. 課金が workspace 所有者の account に乗る。**実行者本人の account 負担**にするには計上時に `requested_by` → account を引く付け替えが要る

## 設計

### 1. DB スキーマ(新規マイグレーション 2 本)

`0016_workspace_members.up.sql` — メンバー招待型

```sql
CREATE TABLE workspace_members (
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL,
  role         TEXT NOT NULL DEFAULT 'viewer',  -- owner|editor|viewer
  invited_by   TEXT NOT NULL,
  invited_at   TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX idx_workspace_members_user ON workspace_members(user_id);
```

`0017_workspace_share_links.up.sql` — 公開リンク型

```sql
CREATE TABLE workspace_share_links (
  token        TEXT PRIMARY KEY,         -- ランダム不可推測トークン
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  role         TEXT NOT NULL DEFAULT 'viewer',
  created_by   TEXT NOT NULL,
  expires_at   TIMESTAMPTZ,              -- NULL=無期限
  revoked_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_share_links_workspace ON workspace_share_links(workspace_id);
```

### 2. 認可の拡張(中核)

`IsWorkspaceAccessible` を **role を返す形に進化**させ、書き込み判定もできるようにする:

- account 経由(owner 相当) **OR** `workspace_members` の role **OR** 有効な公開リンクの role を返す
- 該当無し → アクセス不可

service 層は「読み取り RPC = role 問わず可」「書き込み RPC = editor / owner 必須」で分岐。viewer は書き込み系で `ErrForbidden`。tree / item / document も同じゲートを通るので自動的に共有が効く。

### 3. handler の `Unimplemented` を実装

`InviteMember` / `UpdateMemberRole` / `RemoveMember` / `TransferOwnership` を実装(proto はそのまま使える)。
公開リンク用に `CreateShareLink` / `RevokeShareLink` / `ListShareLinks` を proto に追加。

### 4. 公開リンクのアクセス経路

リンクは無認証で踏める必要があるので、Firebase 認証必須の通常経路とは別に token → workspace 解決のパスを [apps/api/internal/handler/authz.go](../../apps/api/internal/handler/authz.go) に追加。token を context に載せ、`IsWorkspaceAccessible` が token でも通るようにする。

### 5. 課金の本人負担への付け替え

「課金が生じる操作は本人負担」を満たすため:

- **gate**: 課金が生じる write 操作(`StartProcessing` / `ResumeProcessing` 等)は `workspace_members` に **登録済みユーザーとして** editor 以上で入っている場合のみ許可。公開リンク・pending 招待では拒否
- **budget check**: 操作前の budget / quota 判定を、workspace 所有者の account ではなく **`requested_by` 本人の account** に対して行う
- **計上**: usage を記録するとき、`workspace_id` から所有者 account を引くのではなく、**ジョブの `requested_by` から account を引いて `usage_events.account_id` に入れる**。worker → API の usage 報告経路で job の `requested_by` を辿れるようにする
- これにより「他人の workspace でドキュメントを処理したら自分の財布から払う」が成立する。所有者は自分が実行した分だけ負担

### 6. フロント

- メンバー招待 UI は paper-in-paper 構想に沿うなら「共有 paper node」
- 公開リンクは無認証なので `/view/[token]` のような別ルートが必要(`MediShare-spec` の `view/[token]` と同型)

## 実装フェーズ(段階導入)

- **Phase 1**: メンバー招待型(DB `workspace_members` + 認可拡張 + 招待 handler 実装)
- **Phase 2**: 課金の本人負担への付け替え(budget check / usage 計上を `requested_by` の account 軸に変更)
- **Phase 3**: 公開リンク(DB `workspace_share_links` + token 認可経路 + share link RPC、**閲覧専用・課金ゼロ**)
- **Phase 4**: フロント(共有 paper node + `/view/[token]` ルート)

## 実装状況

Phase 1〜4 + 主要な未決事項 2 件まで実装済み。

- **Phase 1〜4**: 完了(招待 / 課金本人負担 / 公開リンク / フロント)
- **被共有 workspace の一覧表示**: 完了。`ListWorkspacesByUser` を account 経由 + `workspace_members` 経由の UNION に変更。招待された member の `ListWorkspaces` に workspace が出るようになった(これが無いと URL 直打ちしか導線が無かった)
- **budget 超過時のゲート**: 完了。`DocumentService.ensureBudgetAvailable` で StartProcessing / ResumeProcessing 前に**本人 account** の `budget_exceeded` を確認し、超過していれば `ErrBillingBudgetExceeded`(ResourceExhausted)。本人だけが止まり所有者は無関係。Phase 2 の本人負担と一貫

## 未決事項(残)

- **pending 招待の扱い**: 課金操作は登録済みユーザーのみに限定する方針なので、未登録 email 招待は「閲覧専用の保留招待」として扱うか、そもそも登録必須にするか。`account_users` が `user_id` 文字列で疎結合なので保留も可能だが手間が増える(現状は登録必須 = 未登録は `ErrNotFound`)
- **storage quota の負担先**: 処理コストは本人 account 負担で確定。アップロード(storage)は現状**所有者持ち**(`CreateDocument` が workspace→account JOIN)。本人 account の quota を消費させるかは保留
- 公開リンク token のローテーション・1 workspace あたりの上限
- `TransferOwnership`: proto 定義済みだが handler 未実装。account 経由の所有モデルとの整合に設計判断が要る
- budget 超過の **UI 表示**: 現状はエラーを返すだけ。フロントでの見せ方は未対応
