# Job Snapshot & Checkpoint Specification

## 状態: 確定 (Confirmed)
最終更新日: 2026-05-07

## 概要
本ドキュメントは、LLM Worker における非同期ジョブの中断・再開を制御するための Snapshot および Checkpoint 機構の技術仕様を定義する。
本機構により、エラー発生時やプリエンプション発生時に、成功済みのステージをスキップして効率的に処理を再開できる。

## 1. データ構造

### チェックポイント・インデックス (Postgres)
`job_stage_checkpoints` テーブルに、ジョブの各ステージの最終状態を保持する。
- **Primary Key**: `(job_id, stage)`
- **status**: `running` | `succeeded` | `failed`
- **gcs_ref**: FUSE マウントパス上の JSON 実体への参照。

### チェックポイント・エンベロープ (JSON)
実際の入出力データは FUSE 上に JSON 形式で保存される。
```json
{
  "schema_version": 1,
  "kind": "synthify.worker_checkpoint",
  "stage": "knowledge_tree",
  "job_id": "job_123",
  "document_id": "doc_456",
  "workspace_id": "ws_789",
  "created_at": "2026-05-07T09:00:00Z",
  "inputs": { ... },
  "outputs": { ... },
  "stats": { ... }
}
```

## 2. ステージの定義

ADK エージェントの特定のツール呼び出しを Checkpoint ステージとして定義する。
- **`briefing`**: `generate_brief` ツールの結果。
- **`knowledge_tree`**: `generate_knowledge_tree` ツールの結果。
- **`persistence`**: `persist_knowledge_tree` ツールの結果。

## 3. 実行制御ロジック (Orchestrator)

Orchestrator はツール実行の前後で以下のフックを処理する。

### 実行前フック (BeforeTool)
1. 実行予定のツールがステージ対象か判定。
2. 対象であれば、FUSE から該当ステージの `CheckpointEnvelope` を読み込む。
3. **バリデーション**:
   - `SchemaVersion` が現行と一致するか。
   - `DocumentID` が現在のコンテキストと一致するか。
4. 全て一致する場合、ツール実行をスキップし、`outputs` を即座に返却。

### 実行後フック (AfterTool)
1. ツール実行が成功し、かつステージ対象である場合。
2. 結果を `CheckpointEnvelope` にラップして FUSE へ保存（`.tmp` -> `rename`）。
3. DB の `job_stage_checkpoints` を `succeeded` として更新。

## 4. エラーハンドリングと再試行

- **不整合の検知**: チェックポイントが読み込めない、またはバリデーションに失敗した場合は、そのステージを「新規実行」として扱い、DB の状態を `running` にリセットする。
- **アトミック性**: FUSE 上の書き込み失敗は、ジョブ自体の致命的な失敗とはせず、ログ出力にとどめる（次回リトライ時に再度実行されるため）。

## 関連ドキュメント
- [gcs-fuse-ingestion-spec.md](gcs-fuse-ingestion-spec.md)
- [llm-worker-architecture.md](llm-worker-architecture.md)
