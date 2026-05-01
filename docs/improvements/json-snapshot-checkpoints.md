# LLM worker の JSON snapshot / checkpoint 設計

## 背景

現状の worker は `text_extraction` → `semantic_chunking` → `goal_driven_synthesis` → `persistence` を 1 回の agent run として実行している。
途中で worker が落ちる、LLM 呼び出しが失敗する、capability 上限に達する、といったケースでは job が failed になり、次回は最初からやり直しになる。

`document_chunks` の再利用 fallback は一部あるが、処理全体として「どこまで完了したか」「どの中間成果物から再開できるか」を表す永続状態がない。

## 方針

- **GCS が正本**: stage ごとの JSON snapshot データを workspace 単位のディレクトリ構造で保管する
- **PostgreSQL は索引のみ**: `job_id / stage / status / gcs_ref` だけ持ち、スキーマはツールが増えても変わらない
- **Firestore**: UI 向け進捗 projection のまま。復帰判定には使わない

ツール固有のフィールドはすべて GCS の JSON に入れる。DB には存在を知らせる最小の索引だけを置く。

## 非目標

- ADK の `SessionService` を永続化して会話履歴から完全復元すること
- LLM の token-by-token 途中状態から再開すること
- tool 呼び出しの途中（例: chunk 10 個中 6 個だけ完了）という細粒度復帰を最初から保証すること

最初の目標は stage 単位の復帰。

## GCS ディレクトリ構造

```
workspaces/{workspace_id}/jobs/{job_id}/
  briefing.json
  critique.json
  synthesis.json
  persistence.json
  ...
```

stage 名がそのままファイル名になる。ツールが増えたらファイルを追加するだけで、他に変更は不要。

## DB テーブル（索引のみ）

```sql
CREATE TABLE IF NOT EXISTS job_stage_checkpoints (
  job_id     TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  stage      TEXT NOT NULL,
  status     TEXT NOT NULL,          -- running | succeeded | failed
  gcs_ref    TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (job_id, stage)
);

CREATE INDEX IF NOT EXISTS idx_job_stage_checkpoints_job_id
  ON job_stage_checkpoints(job_id);
```

`gcs_ref` は `workspaces/{ws_id}/jobs/{job_id}/{stage}.json` の形式。空文字の場合は snapshot なし（running / failed 直後）。

## GCS JSON の envelope

すべての snapshot は共通 envelope を持つ。ツール固有のデータは `outputs` に入れる。

```json
{
  "schema_version": 1,
  "kind": "synthify.worker_checkpoint",
  "stage": "briefing",
  "job_id": "job_123",
  "document_id": "doc_123",
  "workspace_id": "ws_123",
  "created_at": "2026-05-01T10:00:00Z",
  "inputs": {},
  "outputs": {},
  "stats": {}
}
```

| field | 内容 |
|---|---|
| `schema_version` | JSON schema のバージョン。破壊的変更時にインクリメント |
| `kind` | 固定値 `synthify.worker_checkpoint` |
| `stage` | stage 名（ファイル名と一致） |
| `inputs` | stage 入力。再開時の整合性チェックに使う |
| `outputs` | tool 固有の出力。次 stage への引き継ぎデータ |
| `stats` | token 数・duration など任意の計測値 |

## stage 別 outputs の例

### `briefing`
```json
{
  "outputs": {
    "topic": "Service architecture",
    "claim_summary": "...",
    "entities": ["..."],
    "level01_hints": ["..."],
    "outline": ["..."]
  }
}
```

### `synthesis`
```json
{
  "outputs": {
    "items": [
      {
        "local_id": "item_1",
        "label": "Service Architecture",
        "level": 1,
        "description": "...",
        "summary_html": "<p>...</p>",
        "source_chunk_ids": ["chunk_1"]
      }
    ]
  }
}
```

### `persistence`
```json
{
  "outputs": {
    "local_to_item_id": { "item_1": "ti_abc" },
    "created_item_ids": ["ti_abc"]
  }
}
```

## 復帰アルゴリズム

`processDocument` 開始時に checkpoint 一覧を DB から読む。

```
1. DB から job_id の全 checkpoint を取得
2. stage ごとに:
   a. status == succeeded → gcs_ref の JSON を読んでキャッシュ
   b. schema_version が現在コードと一致しない → その stage 以降を再実行
   c. inputs が現在 request と矛盾 → その stage 以降を再実行
   d. 問題なければ skip し、outputs を次 stage の入力に使う
3. 未完了の stage 以降を順に実行
```

## 書き込み順序（atomicity）

```
1. DB を running に upsert
2. stage を実行
3. outputs を JSON に marshal
4. GCS に upload
5. DB を succeeded に update し gcs_ref を保存
```

worker が 4 と 5 の間で落ちた場合、DB は running のまま → 再起動時に未完了扱いで再実行。GCS に孤立した JSON が残るが無害。

persistence stage は DB への副作用が先に起きるため、`created_item_ids` の存在確認で冪等化する。

## Repository / service 追加

```go
type CheckpointRepository interface {
    UpsertStageRunning(ctx context.Context, jobID, stage string) error
    MarkStageSucceeded(ctx context.Context, jobID, stage, gcsRef string) error
    MarkStageFailed(ctx context.Context, jobID, stage, errorMessage string) error
    ListStageCheckpoints(ctx context.Context, jobID string) ([]domain.JobStageCheckpoint, error)
}

type SnapshotStore interface {
    Put(ctx context.Context, ref string, payload []byte) error
    Get(ctx context.Context, ref string) ([]byte, error)
}
```

`SnapshotStore` は GCS emulator / Cloud Storage の両方を同じ interface で扱う。ローカル開発では in-memory 実装も提供する。

## 実装フェーズ

### Phase 1 — 最小実装
- `job_stage_checkpoints` テーブル追加
- `CheckpointRepository` と `SnapshotStore` の interface + 実装
- `briefing` / `synthesis` の snapshot 保存
- `processDocument` を stage orchestration に寄せる

### Phase 2 — persistence の冪等化
- `persist_knowledge_tree` を `created_item_ids` の存在確認で skip できるようにする
- persistence snapshot に `local_to_item_id` の mapping を保存

### Phase 3 — observability
- Firestore progress に `resumed_from_stage` / `checkpointed_stages` を出す
- 失敗 job の snapshot を UI / admin から参照できるようにする

## 関連
- [resume-processing-stub.md](resume-processing-stub.md)
- [generate-execution-plan-hardcoded.md](generate-execution-plan-hardcoded.md)
