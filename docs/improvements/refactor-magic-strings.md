# バックエンドのマジックストリング削減 (Refactor Magic Strings)

## 概要
Worker および API サービス内のビジネスロジックで直接比較・代入されているハードコードされた文字列（マジックストリング）を、`domain` パッケージ等の定数に置き換えます。これにより、コンパイラによる型チェックを効かせ、将来のステータス追加時などにタイポ起因のバグを防ぐことを目的とします。

## 対象となるファイルと具体的な変更点

### 1. ドキュメントのステータス (`DocumentStatus`)
*   **追加する定数:**
    `apps/worker/pkg/worker/domain/document.go` (または `status.go` など適切な場所) に以下を追加します。
    ```go
    type DocumentStatus string
    const (
        DocumentStatusReserved  DocumentStatus = "reserved"
        DocumentStatusConfirmed DocumentStatus = "confirmed"
        DocumentStatusRejected  DocumentStatus = "rejected"
    )
    ```
*   **修正箇所:**
    *   `apps/worker/pkg/worker/repository/postgres/document.go`:
        *   `if status == "confirmed"` -> `if status == string(domain.DocumentStatusConfirmed)`
        *   `if status != "reserved"` -> `if status != string(domain.DocumentStatusReserved)`
        *   `Status: "rejected"` -> `Status: string(domain.DocumentStatusRejected)`
    *   `apps/worker/pkg/worker/repository/mock/store.go`:
        *   モック内の同等の文字列比較・代入を定数に置き換え。

### 2. アイテムのガバナンス状態 (`GovernanceState`)
*   **追加する定数:**
    `apps/worker/pkg/worker/domain/tree.go` に以下を追加します。
    ```go
    const (
        GovernanceStateSystemGenerated NodeGovernanceState = "system_generated"
        GovernanceStatePendingReview   NodeGovernanceState = "pending_review"
        GovernanceStateHumanCurated    NodeGovernanceState = "human_curated"
        GovernanceStateLocked          NodeGovernanceState = "locked"
    )
    ```
*   **修正箇所:**
    *   `apps/worker/pkg/worker/repository/postgres/item.go`:
        *   `if row.GovernanceState == "human_curated" || row.GovernanceState == "locked"` -> `if row.GovernanceState == string(domain.GovernanceStateHumanCurated) || row.GovernanceState == string(domain.GovernanceStateLocked)`

### 3. Worker のアクション結果ステータス
*   **追加する定数:**
    `apps/worker/pkg/worker/domain/job.go` (または `worker.go` などの適切な場所) に以下を追加します。（すでに類似の型があればそれを利用します）
    ```go
    type ActionStatus string
    const (
        ActionStatusRejected ActionStatus = "rejected"
        // 必要に応じて pending, completed 等も追加
    )
    ```
*   **修正箇所:**
    *   `apps/worker/pkg/worker/worker.go`:
        *   `if a.Status == "rejected"` -> `if a.Status == string(domain.ActionStatusRejected)`

### 4. Memory/Journal ツールのステータス
*   **追加する定数:**
    `apps/worker/pkg/worker/tools/builtin/memory/journal.go` ファイル内で自己完結しているため、同ファイル内に以下を追加します。
    ```go
    type JournalStatus string
    const (
        JournalStatusPending    JournalStatus = "pending"
        JournalStatusInProgress JournalStatus = "in_progress"
        JournalStatusCompleted  JournalStatus = "completed"
    )
    ```
*   **修正箇所:**
    *   同ファイル内の構造体定義: `Status string` -> `Status JournalStatus`
    *   switch 文の条件: `case "completed":` -> `case JournalStatusCompleted:`

## 検証方法
1.  **コンパイル:**
    `go build ./...` を実行し、型エラーやパッケージ間の循環参照が発生しないことを確認します（特に `sqlc` の生成コードが期待する `string` 型へのキャスト漏れがないか確認）。
2.  **テスト実行:**
    `cd apps/worker && go test ./...` を実行し、既存のモックストアや DB 接続テストがすべてパスすることを確認します。