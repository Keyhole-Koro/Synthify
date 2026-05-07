# ワークスペース tree のライフサイクル — 複数ドキュメント処理時の統合仕様（ドラフト）

## 背景

現状の worker は 1 ドキュメントを処理するたびに新規 items を追記するだけで、既存 tree との関係を考慮しない。
ワークスペースに複数のドキュメントを順次追加していくユースケースでは、以下の問題が生じる：

- 同じ概念が複数ドキュメント由来で重複 item として存在する
- 後から追加したドキュメントの知識が既存 tree の構造を改善できない
- 再処理時に前回生成 items の扱いが未定義

---

## フェーズ構成

### Phase 1 — 既存 tree の読み取り（read-only context）

**目的**: synthesize 品質の向上。統合・更新はしない。

**変更点**:
- `ExecuteApprovedPlan` の処理開始時に、ワークスペースの既存 tree items をロードする
- セマンティック検索（`semantic_search` ツール）で処理中のチャンクに関連する既存 items を引いてくる
- LLM の synthesize プロンプトに既存 tree の概要を context として渡す
- 出力は CREATE_ITEM のみ。既存 item への変更はしない

**open question**:
- 既存 items を全件渡すか、関連するものだけ渡すか（トークン上限の問題） 
- 関連 items の検索は chunk の embedding で引くか、item の description で引くか

---

### Phase 2 — 新規 item の適切な挿入位置の決定

**目的**: 新規 item をルート直下に並べるのではなく、既存 tree の適切な親の下に挿入する。

**変更点**:
- synthesize 時に既存 items の中から親候補を LLM に選ばせる
- `persist_knowledge_tree` ツールが `parent_local_id` のほかに `parent_existing_item_id` を受け取れるようにする
- capability に `ATTACH_TO_EXISTING_ITEM` 相当の権限概念を追加するか検討

**open question**:
- 親として指定できる items の範囲を capability でどう制約するか
- 挿入先が見つからない場合のフォールバック（ルート直下）

---

### Phase 3 — 既存 item の update / merge

**目的**: 後から処理したドキュメントが既存 item の description・summary・構造を改善できるようにする。

**変更点**:
- `UPDATE_ITEM` operation を capability に含める
- worker が「この item は既存の X と同一概念」と判断したとき、X を update または X に source を追加する
- item の provenance（`source_chunk_ids`）が複数ドキュメント由来になることを許容する
- `UpsertItemSource` は既に複数 source を許容しているのでそのまま使える

**open question**:
- 同一概念の判断基準（embedding 類似度の閾値？ LLM 判断？）
- update の権限: 「他のドキュメントの job が生成した item を別 job が更新していい」条件
- 構造変更（親子関係の組み替え）は Phase 3 に含めるか Phase 4 に分けるか

---

## 再処理時のルール（未決定）

同じドキュメントを再処理したとき、前回生成 items をどう扱うか：

| 方針 | メリット | デメリット |
|------|---------|-----------|
| 追記のみ（現状） | シンプル | 重複が増える |
| ドキュメント単位でリセット | クリーン | 他ドキュメントが参照していた items が消える |
| upsert（冪等キー） | 重複しない | 冪等キーの設計が難しい |
| 論理バージョン管理 | 履歴が残る | クエリが複雑になる |

Phase 1〜2 の実装後に実態を見て決める。

---

## 現状のギャップ（実装前に解決が必要なもの）

- `GetWorkspaceTreeItems(ctx, workspaceID)` 相当の repository メソッドがない
---

## 実装提案 (Audit 結果に基づく Phase 2 移行案)

現状の「単発ドキュメント処理」から「継続的な知識統合」へ移行するための具体的修正案。

### 1. ツールスキーマの拡張

#### `goal_driven_synthesis` (process/synthesis.go)
- **変更内容**: 引数に `existing_tree_context` (既存の主要な項目名と ID のリスト) を追加。
- **目的**: LLM が「新しい情報は既存のどの項目の下に置くべきか」を推論できるようにする。

#### `persist_knowledge_tree` (io/persistence.go)
- **変更内容**: `SynthesizedItem` 構造体に `parent_existing_item_id` (string) を追加。
- **ロジック**: `parent_local_id` が空で、かつ `parent_existing_item_id` が指定されている場合、ワークスペースの既存アイテムを親として紐付ける。

### 2. Orchestrator の知識注入 (agents/orchestrator.go)

- **初期化フェーズの追加**:
  ```go
  // ジョブ開始時にワークスペースの全アイテム（または主要な骨組み）を取得
  items := repo.GetTreeByWorkspace(ctx, workspaceID)
  // 既存ツリーの構造を Working Memory の一部として LLM に提供
  // 例: "Existing Tree Structure: [id:1] Overview, [id:2] Technical Specs..."
  ```
- **プロンプトの更新**: 「新しい概念を見つけたとき、それが既存の項目 X の詳細であるなら、X を親として指定せよ」という指示を強化。

### 3. Capability の権限追加

- `allowed_operations_json` に `UPDATE_EXISTING_ITEM` を追加。
- これにより、後続のドキュメントが「既存アイテムの要約を、より正確な内容に上書きする」ことを許可する。

### 4. 解決すべき技術的課題

- **コンテキスト溢れ**: 既存ツリーが巨大になった場合、全アイテムをプロンプトに入れることはできない。
  - **対策**: `semantic_search` を活用し、現在処理中のチャンクに類似した既存アイテムのみを「親候補」として動的に提示する。
- **冪等性の確保**: 再処理時に同じ親に同じ子が何度もぶら下がるのを防ぐため、`provenance` (どのドキュメント由来か) を使った重複チェックを `persist_knowledge_tree` に実装する。
