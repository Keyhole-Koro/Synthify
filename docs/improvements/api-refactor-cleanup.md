# API 層の仕様変更残骸とリファクタリング候補

## ステータス確認 (2026-05-07)

現時点での進捗状況：
- **P1, P2, P3**: すべて **未着手 (Pending)**。挙動の不整合や古い RPC が残っている。
- **P4**: 旧パイプライン残骸が **未着手 (Pending)**。ラッパー削除は完了。

## 背景

認証・アップロード・workspace membership・Firestore presence・LLM worker job 周りの仕様変更後、API / proto / mapper / service に古い前提が残っている。
このメモは、現時点で確認できた API 層の整理候補を優先度付きでまとめる。

---

## P1 — 挙動不整合・バグ候補

### 1. `CreateProcessingJob` に `tree.TreeID` を `workspaceID` 引数として渡している

**場所**

- `api/internal/service/document.go` — `StartProcessing`
- `api/internal/service/document.go` — `ResumeProcessing`

**現状**

```go
tree, err := s.tree.GetOrCreateTree(ctx, wsID)
...
job := s.repo.CreateProcessingJob(ctx, documentID, tree.TreeID, jobType)
```

**修正方針**

呼び出しは `wsID` を渡す。

---

### 2. `Document.updated_at` が実データを持っていない

**場所**

- `shared/mappers/mappers.go` — `ToProtoDocument`
- `shared/domain/types.go` — `Document`

**修正方針**

DB / domain に `documents.updated_at` を追加して mapper で返す。

---

### 3. `Job.completed_at` が実完了時刻ではない

**場所**

- `shared/mappers/mappers.go` — `ToProtoJob`
- `shared/domain/types.go` — `DocumentProcessingJob`

**修正方針**

DB / domain に `started_at` / `completed_at` を追加し、job lifecycle 更新時に明示的に書く。

---

## P2 — 重複コード・削除候補

### 4. `StartProcessing` と `ResumeProcessing` の dispatch ロジックが重複している

**場所**: `api/internal/service/document.go`

**修正方針**: private helper に抽出する。

---

### 5. `GetUploadURL` RPC が現行 upload flow と二重化している

**修正方針**: `DocumentService.GetUploadURL` 関連を削除する。

---

### 6. Workspace membership RPC が account-level 管理に移行済み

**修正方針**: workspace proto から削除または deprecated 化。

---

### 7. Item activity RPC が Firestore presence に移行済み

**修正方針**: proto / handler から削除または deprecated 化。

---

### 8. `DocumentService.GetLatestProcessingJob` は service public method として不要そう

**修正方針**: 外部利用がなければ削除する。

---

## P3 — 型・契約の整理

### 9. `domain.TreeItem` が未使用に見える

**修正方針**: 参照がないことを確認したうえで削除する。

---

## P4 — 余計なラッパー関数・不要な間接レイヤー

### 10. `worker/pkg/worker/pipeline` パッケージが旧アーキテクチャの残骸

**場所**: `worker/pkg/worker/pipeline/`

ADK エージェント移行後、このパッケージは **一切インポートされていない**。

**After**

```
worker/pkg/worker/pipeline/ ディレクトリごと削除
```

---

## 実装順の提案

1. [未着手] `CreateProcessingJob` 呼び出しを `tree.TreeID` から `wsID` に修正し、テストを追加する（バグ修正）
2. [未着手] `Document.updated_at` と `Job.completed_at` の contract を決める（proto 設計）
3. [未着手] `StartProcessing` / `ResumeProcessing` の dispatch helper 抽出
4. [未着手] `worker/pkg/worker/pipeline/` ディレクトリ削除（依存なしで安全）
5. [未着手] `GetUploadURL` / workspace membership / item activity RPC を削除または deprecated 化する
6. [未着手] 未使用 domain 型（`TreeItem`）を削除する
