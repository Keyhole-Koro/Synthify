# Dynamic Tool Pipeline — 未着手タスクの集約

worker が生成した dynamic tool が「**人間が承認 → 本番で再利用 → eval で品質維持**」までを通る
パイプライン全体の残作業を 1 ファイルにまとめる。設計の正本ではなく**作業リスト**として読む
（設計判断はコードの doc コメント・各 contract に保全済み）。

## 全体像

```
worker (本番処理)                    log-viewer (人間)              eval (バッチ)
─────────────────                  ──────────────                ──────────────
  agent が文書を処理                  candidate 一覧                 active な dynamic tool を
       │                              │                            固定 case で回帰実行
       │ create_transform で           │ approve / reject            │
       │ dynamic tool を生成            │ ボタン                       │ LLM が改善 Code を提案
       ▼                              ▼                            ▼
  candidate を DB に保存 ───────────▶ status を active/rejected ───▶ eval report → 人間 review
       │                                                            │
       │ 次の job で resolve                                          │ 採用された改善は
       ▼                                                            ▼
  active を agent に合流                                            log-viewer で再承認
```

3 アプリの責務:
- **worker** = 生成 + 実行 (本番ランタイム)
- **log-viewer / BI** = 人間が承認する場所
- **eval** = バッチ品質管理 (回帰チェック + 改善提案 LLM)

データフローのハブは **Postgres `dynamic_tools` テーブル**（status: candidate / active / held / rejected / disabled）。

---

## 現状 (2026-05-20)

### 動いているもの
- worker の道A 化（builtin/dynamic を同一 `sharedtool.Tool` 型で扱う、`adkadapter.Wrap` で ADK 接続を 1 点集約）
- worker per-job agent 再構築 (`ProcessDocument` で `DynamicToolRepository.ResolveActiveTools` → wrap → 新 llmagent)
- `create_transform` メタツール（Starlark エフェメラル実行のみ）
- `domain.DynamicTool` model + postgres リポジトリ全 API（`RecordCandidate` / `ListCandidates` /
  `PromoteCandidate` / `PromoteToGlobal` / `ResolveActiveTools`）
- eval 道A 化（knowledge_tree のみ、JSON Schema rule、`--variant` で prompt 比較）

### 動いていないもの
- worker: `create_transform` 成功時に `RecordCandidate` を呼ぶ配線がない → **dynamic tool が DB に保存されない**
- log-viewer: candidate を表示する UI / approve API がない → **人間が承認できない**
- worker: 結果として active な dynamic tool が常に空 → per-job 再構築は動いても合流するものがない
- eval: `apps/eval/runner/dynamic.go` は stub → **eval で dynamic tool を評価できない**
- 改善提案 LLM (prompt 用・dynamic tool 用) はゼロ
- Cloud Run Job の定時バッチもなし

---

## 残タスク

### Phase 1: worker DB 配線（candidate 記録）

**目的**: `create_transform` が成功した dynamic tool を DB に candidate として永続化する。
これがないとパイプライン全体が始まらない（pipeline の起点）。

**やること**:
- `process.CreateTransformTool` の Run の最後（StatusOK 時）に `repo.RecordCandidate(...)` 呼び出しを追加
- candidate 用 fields の値決定:
  - `Name`: `args.Name`
  - `Code`: `args.Code`
  - `Language`: `args.Language`
  - `IOSchema`: 規約は `sharedtool.IOSchemaFromJSON` の wrapper 形（`{"input":..., "output":...}`）。
    `create_transform` 引数に IOSchema を追加するか、初回 candidate は input サンプル + output から推定して保存するか要決定
  - `DeclaredTier`: LLM 自己申告 (`create_transform` に optional 引数追加)
  - `FloorTier`: `analyzer.Analyze` の結果から取得
  - `RiskTier`: `max(declared, floor)`（`util.NormalizeRiskTier`）
  - `OriginWorkspaceID`, `OriginJobID`: `base.Context.Job` から
  - `InputSample`: `args.InputSample`
  - `Status`: 常に `candidate`
- worker.Repository に `RecordCandidate` を含む契約はすでに満たされているか確認（DynamicToolRepository が追加済み）

**スコープ外**: 重複検知（同 description 連続生成抑制）。後続で。

**前提**: dynamic-tool-synthesis.md の旧仕様は削除済み。設計根拠が要るなら git 履歴参照。

---

### Phase 2: log-viewer 承認 UI

**目的**: candidate を一覧表示し、人間が approve / reject する画面。これがないと active 状態に
遷移しない。

**やること**:
- `apps/log-viewer` (Next.js) に新ページ追加:
  - `candidate` 一覧（DB から read-only 取得）
  - 各行に「approve」「reject」「held (tier_3 用)」「kill switch (active→disabled)」ボタン
  - 詳細パネル: Code / IOSchema / InputSample / FloorTier / DeclaredTier / OriginWorkspaceID
- BFF route handler:
  - `POST /api/dynamic-tools/{id}/promote` → `repo.PromoteCandidate(id, "active")`
  - `POST /api/dynamic-tools/{id}/reject` → status を rejected に
  - `POST /api/dynamic-tools/{id}/disable` → kill switch
- 認証: [admin-dashboard-security.md](admin-dashboard-security.md) の方針に従う

**スコープ外**: tier 別の自動昇格（tier_1 即 active）。初期は全件 human review。

**依存**: Phase 1（candidate が DB に存在することが前提）

---

### Phase 3: worker dynamic resolve

**状態**: ✅ **完了済み**（今セッション）。Phase 2 で active な tool が生まれれば自動で次 job 以降の
agent に合流する。残作業なし。

---

### Phase 4: eval dynamic 評価

**目的**: active な dynamic tool を eval runner で固定 input に対して回帰実行し、io_schema 適合と
決定論性を検証する。worker での昇格前ゲート ([dynamic-tool-synthesis 旧仕様の tier1=回帰] に相当)。

**やること**:
- `apps/eval/runner/dynamic.go` の `newDynamicTool` stub を本実装に
  - `transform.Engine.Execute(Code, Input)` 呼び出し（worker の `tools/dynamic/dynamic.go` と同等の logic）
  - `IOSchemaFromJSON` で input/output schema parse（worker と同じ規約）
- `DynamicToolSource` の本実装 (eval 側の repository 注入)
- eval `Case` YAML に `tool: <dynamic tool name>` を書ける形式拡張
- 決定論性チェック: 同 input 2 回実行で同 output か検証する CaseExpect op 追加 (`deterministic` 等)

**依存**:
- Phase 1（DB に candidate がある）
- Phase 2（active になった tool がある）
- Phase D 推奨（eval と worker で sharedtool 型を共有してから dynamic 配線すると重複が減る）

---

### Phase 5: 改善提案 LLM

#### 5a. Prompt 改善 LLM（既存 prompt-variant-eval-contract の延長）

**目的**: eval report と golden diff（または期待出力との乖離）を Analyst LLM が読み、改善方針を
構造化出力。Prompt Writer LLM が variant prompt を生成。

**やること**:
- `apps/eval/analysis/` パッケージ新設
- Analyst CLI: `synthify-eval analyze --report <gs://path>` → `analysis.json` 出力
- Prompt Writer CLI: `synthify-eval write-prompt --analysis <path>` → `apps/eval/variants/generated/<ts>/`
- 改善案は人間 review (log-viewer) を経て採用

**前提**: golden 機能は道A で削除済み。回帰判定は JSON rule + schema 適合 + LLM 主観評価のどれを
根拠にするか先に decision。

#### 5b. Dynamic tool 改善 LLM

**目的**: eval で回帰失敗 / 性能劣化した dynamic tool に対し、Code 改善案を LLM が生成。

**やること**:
- 入力: `DynamicTool.Code` + 失敗 input/output + 期待結果
- 出力: 改善 Code + 改善理由
- 出力は新 candidate として記録（既存 tool は active のまま、新 version で並行）
- 人間 review → 採用なら旧 tool を `superseded` に、新を active に

**依存**: Phase 4（eval 評価が動いている）

---

### Phase 6: Cloud Run Job 定時バッチ

**目的**: Phase 4 + 5 を定期実行（cron）。常時人間が手動実行する状況をなくす。

**やること**:
- `apps/eval/Dockerfile` を Cloud Run Job として稼働させる Terraform (`terraform/services/eval` 参照)
- Cloud Scheduler で cron 設定（毎日 / 毎週）
- GCS artifact 保存（[llm-eval-gcs-artifact-contract.md](../contracts/llm-eval-gcs-artifact-contract.md) 既存）
- 失敗時 Slack 通知（CRITICAL severity 経路があれば流用、なければ別途設計）

**依存**: Phase 4, 5

---

### Phase D: shared/tool 統合（型重複の解消）

**目的**: 道A の `Tool/RunFn/IOSchema/Usage` 型が現在 eval (`apps/eval/runner/tool.go`) と worker
(`apps/worker/pkg/worker/tools/sharedtool/`) に二重定義されている。`packages/shared/tool` に集約し
両者が import する形にする。

**やること**:
- `packages/shared/tool/` パッケージ新設（worker sharedtool のコピー）
- worker sharedtool を `import "github.com/synthify/backend/packages/shared/tool"` の re-export か削除
- eval の `apps/eval/runner/tool.go` の Tool / RunFn / IOSchema / Usage を削除して shared を import
- adkadapter / dynamic / runner / builtin_knowledge_tree 等の参照を全部更新
- 既存テスト追従

**スコープ外**: 機能変更なし（純粋な refactor）

**依存**: なし（独立してできる）

**優先度**: 低（並存しても build/test 通る、保守時に「どちらの型?」と迷う原因なので時間ある時に）

---

## Phase 間の依存関係

```
Phase 1 (worker DB 配線)
  ├─▶ Phase 2 (log-viewer 承認)
  │      └─▶ Phase 3 [完了済] (worker resolve)
  │            └─▶ Phase 4 (eval dynamic 評価)
  │                  └─▶ Phase 5b (dynamic 改善 LLM)
  │                         └─▶ Phase 6 (定時バッチ)
  └─ Phase 5a (prompt 改善 LLM) は Phase 1 独立で動かせる
              └─▶ Phase 6

Phase D (shared/tool 統合) はどの Phase ともクロスせず独立
```

**最短経路**: Phase 1 → Phase 2 → (Phase 3 完了) → Phase 4 → Phase 5b → Phase 6
（5a は別ルートで並行可）

## 関連の既存ドキュメント

- 道A の設計根拠: `apps/worker/pkg/worker/tools/sharedtool/doc.go`,
  `apps/eval/runner/tool.go` の doc コメント
- prompt 変異 eval: [../contracts/prompt-variant-eval-contract.md](../contracts/prompt-variant-eval-contract.md)
- eval runner: [llm-eval-runner.md](llm-eval-runner.md)
- log-viewer BI: [log-viewer-bi-dashboards.md](log-viewer-bi-dashboards.md)
- log-viewer 認証: [admin-dashboard-security.md](admin-dashboard-security.md)
- 旧 dynamic-tool-synthesis.md / transform-engine-registry.md は git 履歴に存在（設計再考のため削除済み）
