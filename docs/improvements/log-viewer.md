# Log Viewer — 設計計画

## 概要

`log-viewer` サブモジュール（`github.com/Keyhole-Koro/SynthifyLogViewer`）が
ロガー、イベントスキーマ、ジョブ観測 UI を所有する。

既存の Web 側 `AgentTraceViewer` は最終的に廃止し、LLM worker trace UI も
`log-viewer` の `JobLogViewer` に完全移行する。Go 側の書き込みと React 側の表示を
同じリポジトリで保守し、イベント種別や表示ロジックのズレを防ぐ。

---

## 既存システムとの棲み分け

| システム | ストレージ | 用途 |
|----------|-----------|------|
| `jobstatus` (Firestore) | Firestore | リアルタイムのジョブ状態（UI の進捗バー等） |
| `AgentTraceViewer` (現行) | `job_mutation_logs` | エージェントのツール呼び出し・input/output |
| **Log Viewer (移行先)** | `job_logs` + `job_mutation_logs` | ツール trace、システムイベント、エラー、LLM レイテンシ |

`AgentTraceViewer` は一時的な互換 UI として扱う。新規 UI 実装は `log-viewer/ui` に寄せ、
移行完了後に `web/src/features/tree/AgentTraceViewer.tsx` を削除する。

---

## log-viewer サブモジュール構成

```
log-viewer/
├── README.md
├── go.mod                     — Go パッケージ (module: github.com/Keyhole-Koro/SynthifyLogViewer)
├── logger.go                  — Logger インターフェース + Event スキーマ
├── noop.go                    — テスト用 NoopLogger
└── ui/
    ├── package.json
    └── src/
        ├── JobLogViewer.tsx   — 統合 React コンポーネント
        ├── views/
        │   ├── TimelineView.tsx
        │   ├── FlowView.tsx
        │   └── DetailPanel.tsx
        └── types.ts           — UI 側イベント型
```

---

## イベントスキーマ

### DB テーブル: `job_logs`

```sql
CREATE TABLE job_logs (
    id          TEXT        PRIMARY KEY,
    job_id      TEXT        NOT NULL,
    workspace_id TEXT       NOT NULL,
    document_id TEXT        NOT NULL DEFAULT '',
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    level       TEXT        NOT NULL,  -- INFO / WARN / ERROR
    event       TEXT        NOT NULL,  -- ドット区切りキー (例: job.queued)
    message     TEXT        NOT NULL,
    detail_json JSONB       NOT NULL DEFAULT '{}'
);
CREATE INDEX job_logs_job_id_ts_idx ON job_logs (job_id, timestamp);
CREATE INDEX job_logs_document_id_ts_idx ON job_logs (document_id, timestamp);
CREATE INDEX job_logs_workspace_id_ts_idx ON job_logs (workspace_id, timestamp);
CREATE INDEX job_logs_level_ts_idx ON job_logs (level, timestamp);
```

検索を強く使うなら、Postgres の `tsvector` または trigram index を追加する。
初期実装では `message`、`event`、`detail_json::text` に対する `ILIKE` で十分だが、
件数が増えたら以下を検討する。

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX job_logs_search_trgm_idx
ON job_logs USING gin ((event || ' ' || message || ' ' || detail_json::text) gin_trgm_ops);
```

### イベントキー一覧

| event | level | detail のキー例 |
|-------|-------|----------------|
| `job.queued` | INFO | `type` |
| `job.running` | INFO | — |
| `job.completed` | INFO | — |
| `job.failed` | ERROR | `error` |
| `job.dispatch_failed` | ERROR | `error` |
| `llm.call.completed` | INFO | `model`, `duration_ms` |
| `llm.usage_exceeded` | ERROR | `resource`, `count`, `limit` |
| `chunks.saved` | INFO | `count` |
| `evaluation.completed` | INFO | `passed`, `score`, `findings` |
| `approval.requested` | INFO | `by`, `reason` |
| `approval.approved` | INFO | `by`, `approval_id` |
| `approval.rejected` | WARN | `by`, `approval_id`, `reason` |
| `tool.call.started` | INFO | `tool`, `input` |
| `tool.call.completed` | INFO | `tool`, `duration_ms`, `output` |
| `tool.call.failed` | ERROR | `tool`, `duration_ms`, `error`, `input` |

`tool.call.*` は現行の `job_mutation_logs` から移行するイベント。移行期間中は
`job_mutation_logs` を読み取り source として残し、viewer 側で unified event に正規化する。

---

## Go パッケージ設計

### `logger.go`

```go
package joblog

type Level string
const (
    INFO  Level = "INFO"
    WARN  Level = "WARN"
    ERROR Level = "ERROR"
)

type Event struct {
    JobID      string
    WorkspaceID string
    DocumentID string
    Level      Level
    Event      string
    Message    string
    Detail     map[string]any // optional
}

type Logger interface {
    Log(ctx context.Context, e Event)
}
```

- `Log` は非同期・fire-and-forget（エラーは stdout に出すだけでジョブを止めない）
- stdout ミラー込みで実装する（`log.Printf` を内部で呼ぶ）

### `noop.go`

テスト・ローカル開発用の何もしない実装。

---

## 実装側（DB 書き込み）の置き場

`shared/repository` に `LogJobEvent(ctx, event)` メソッドを追加。
`log-viewer` の `Logger` インターフェースを実装する `DBLogger` を
`shared/repository/postgres/` に置く。

```
shared/
└── repository/
    ├── joblog.go                  — LogJobEvent インターフェース定義
    └── postgres/
        └── joblog.go             — DBLogger (log-viewer の Logger を実装)
```

---

## API 変更

新規 RPC を追加する。

`ListJobLogs` は最終的に `job_logs` と `job_mutation_logs` の両方を統合して返す。
これにより Web UI は `ListJobMutationLogs` を直接呼ばなくなる。

```proto
rpc ListJobLogs(ListJobLogsRequest) returns (ListJobLogsResponse);
rpc SearchJobLogs(SearchJobLogsRequest) returns (SearchJobLogsResponse);
rpc ListRelatedJobLogs(ListRelatedJobLogsRequest) returns (ListRelatedJobLogsResponse);

message ListJobLogsRequest {
  string job_id = 1;
  int32 page_size = 2;
  string page_token = 3;
}

message ListJobLogsResponse { repeated JobLog logs = 1; }

message JobLog {
    string timestamp   = 1;
    string level       = 2;
    string event       = 3;
    string message     = 4;
    string detail_json = 5;
    string source      = 6; // system / tool
    string source_id   = 7; // job_logs.id or job_mutation_logs.mutation_id
    string job_id      = 8;
    string document_id = 9;
    string workspace_id = 10;
}

message SearchJobLogsRequest {
  string query = 1;
  string workspace_id = 2;
  string document_id = 3;
  string job_id = 4;
  repeated string levels = 5;
  repeated string events = 6;
  string from_timestamp = 7;
  string to_timestamp = 8;
  int32 page_size = 9;
  string page_token = 10;
}

message SearchJobLogsResponse {
  repeated JobLog logs = 1;
  string next_page_token = 2;
}

enum RelatedLogScope {
  RELATED_LOG_SCOPE_UNSPECIFIED = 0;
  RELATED_LOG_SCOPE_JOB = 1;
  RELATED_LOG_SCOPE_DOCUMENT = 2;
  RELATED_LOG_SCOPE_WORKSPACE = 3;
}

message ListRelatedJobLogsRequest {
  string job_id = 1;
  string document_id = 2;
  string workspace_id = 3;
  RelatedLogScope scope = 4;
  int32 page_size = 5;
  string page_token = 6;
}

message JobLogGroup {
  string workspace_id = 1;
  string document_id = 2;
  repeated JobLogJob jobs = 3;
}

message JobLogJob {
  string job_id = 1;
  JobLifecycleState status = 2;
  string created_at = 3;
  repeated JobLog logs = 4;
}

message ListRelatedJobLogsResponse {
  repeated JobLogGroup groups = 1;
  string next_page_token = 2;
}
```

移行後の扱い:

- `ListJobMutationLogs` は internal / legacy API として残すか、deprecated 化する。
- `web/src/features/tree/api.ts` の `listJobMutationLogs` は削除する。
- Web は `log-viewer` の data adapter 経由で `ListJobLogs` を呼ぶ。

---

## 検索と関連ログ取得

Log Viewer は「1 job のログを見る」だけではなく、関連 resource をたどってログを読む。

対象にする関係:

```text
workspace 1:n documents
document  1:n jobs
job       1:n logs
job       1:n tool mutation logs
```

代表的なユースケース:

- ある document に対する全 processing / reprocessing job を時系列で見る
- 失敗した job の前後に、同じ document で成功した job があるか確認する
- workspace 全体で ERROR / WARN だけ検索する
- `llm.usage_exceeded` や `job.dispatch_failed` のような event key で絞る
- tool 名、chunk ID、item ID、document ID など `detail_json` 内の値で検索する

### API の役割

`ListJobLogs(job_id)`:

- 1 job の詳細ビュー用
- Audit ページで job を選んだときに使う
- system event と tool event を同じ timeline に統合して返す

`SearchJobLogs(...)`:

- 横断検索用
- `query` は `event` / `message` / `detail_json` / tool input-output の軽量検索に使う
- `workspace_id` は authorization と検索範囲制限に必須扱いにしてよい
- `document_id` / `job_id` / `levels` / `events` / time range で絞り込む

`ListRelatedJobLogs(...)`:

- 1:n 関係を UI に出すための structured API
- `scope=DOCUMENT` なら `document_id -> jobs -> logs`
- `scope=WORKSPACE` なら `workspace_id -> documents/jobs -> logs`
- `scope=JOB` なら `job_id` の周辺情報付き詳細

### Read model

API は `job_logs` と `job_mutation_logs` をそのまま UI に出さず、`JobLog` に正規化する。

`job_logs` 由来:

```text
source      = "system"
source_id   = job_logs.id
event       = job_logs.event
detail_json = job_logs.detail_json
```

`job_mutation_logs` 由来:

```text
source      = "tool"
source_id   = job_mutation_logs.mutation_id
event       = "tool.call.completed" または "tool.call.failed"
detail_json = {
  "tool": target_id,
  "target_type": target_type,
  "mutation_type": mutation_type,
  "risk_tier": risk_tier,
  "input": before_json,
  "output": after_json,
  "provenance": provenance_json
}
```

document / workspace 関連は `document_processing_jobs` を join して埋める。

```sql
SELECT ...
FROM document_processing_jobs j
LEFT JOIN job_logs l ON l.job_id = j.job_id
LEFT JOIN job_mutation_logs m ON m.job_id = j.job_id
WHERE j.document_id = $1
ORDER BY timestamp ASC;
```

実装では `UNION ALL` で `job_logs` と `job_mutation_logs` を統合し、timestamp で sort する。

### Authorization

検索 API は情報量が多いので、必ず workspace access で絞る。

方針:

- `job_id` 指定時: job -> document/workspace を解決して `authorizeDocument`
- `document_id` 指定時: document -> workspace を解決して `authorizeDocument`
- `workspace_id` 指定時: `authorizeWorkspace`
- `workspace_id` なしの横断検索は admin-only にするか、初期実装では禁止する

### UI

`JobLogViewer` は 3 つの表示モードを持つ。

```text
Timeline   1 job の system/tool event を時系列表示
Flow       tool call を Orchestrator root 配下に表示
Related    document -> jobs -> logs の 1:n:n 表示
```

検索 UI:

- 検索ボックス
- level filter: INFO / WARN / ERROR
- event filter: `job.*`, `llm.*`, `tool.*`, `approval.*`
- scope selector: This job / This document / This workspace
- time range

Related view:

```text
Document: doc_123
  Job: job_a  succeeded
    12:00 job.running
    12:01 tool.call.completed semantic_chunking
  Job: job_b  failed
    12:05 job.running
    12:06 llm.usage_exceeded
    12:06 job.failed
```

---

## 統合箇所

以下の `log.Printf` をすべて `logger.Log(ctx, event)` に差し替える。

| ファイル | イベント |
|---------|---------|
| `api/internal/service/document.go` | `job.queued`, `job.dispatch_failed` |
| `api/internal/handler/job.go` | `approval.*` |
| `worker/pkg/worker/worker.go` | `job.running`, `job.completed`, `job.failed`, `evaluation.completed` |
| `worker/pkg/worker/llm/gemini.go` | `llm.call.completed` |
| `worker/pkg/worker/tools/base/usage.go` | `llm.usage_exceeded` |
| `shared/repository/postgres/document.go` | `chunks.saved` |

Logger は context から取得する（`joblog.FromContext(ctx)`）。
Worker.Process でジョブ開始時に注入し、全連鎖に流れる。

既存の tool trace は `agents.Orchestrator.AfterToolCallbacks` から記録されている
`LogToolCall` を source とする。短期的には `job_mutation_logs` を unified log に変換して表示し、
中期的には `LogToolCall` 自体を `log-viewer` logger の `tool.call.*` に寄せる。

---

## UI コンポーネント

### `JobLogViewer.tsx` の役割

- `ListJobLogs(jobId)` を 3 秒ポーリング
- level に応じた色分け（ERROR=赤、WARN=黄、INFO=グレー）
- event キーでフィルタリング可能
- tool call の input/output detail を展開表示
- flow view と timeline view を切り替え可能
- system event と tool event を同じ時系列に並べる

現行 `AgentTraceViewer` の機能は `JobLogViewer` に吸収する。

| 現行 `AgentTraceViewer` 機能 | 移行先 |
|-----------------------------|--------|
| Flow / List 切り替え | `JobLogViewer` の Flow / Timeline view |
| tool input/output 展開 | `DetailPanel` |
| 3 秒 polling | `JobLogViewer` data adapter |
| duration 表示 | `tool.call.completed.detail.duration_ms` |
| Orchestrator root 表示 | `FlowView` の virtual root |

### 既存 Audit ページへの統合

`web/app/audit/page.tsx` は `AgentTraceViewer` を import せず、`log-viewer/ui` の
`JobLogViewer` を直接使う。

```tsx
import { JobLogViewer } from '@synthify/log-viewer/ui';

<JobLogViewer jobId={selectedJobId} />
```

Audit ページ側は job 選択と認証だけを担当し、trace 表示の UI ロジックは持たない。

移行完了後に削除する Web 側コード:

- `web/src/features/tree/AgentTraceViewer.tsx`
- `web/src/features/tree/api.ts` の `listJobMutationLogs`
- audit ページ内の trace 表示専用 CSS / 状態管理

---

## 運用上の考慮事項

### ログの保存期間とクリーンアップ

`job_logs` はジョブ実行のたびに大量に生成されるため、無制限に保存すると DB ストレージを圧迫する。

- **保存期間**: 30日間（デフォルト）。Audit 用途として長期間必要なものは、別途 GCS 等にエクスポートするか、要件に応じて期間を調整する。
- **クリーンアップ**: 毎日深夜に古いログを削除するジョブ（CronJob 等）を実行する。
  ```sql
  DELETE FROM job_logs WHERE timestamp < NOW() - INTERVAL '30 days';
  ```
- **パーティショニング**: ログ件数が数千万件を超える場合は、`timestamp` によるテーブルパーティショニングを検討する。これにより、古いログの削除が `DROP TABLE` で高速に行えるようになる。

### 書き込みパフォーマンス

`logger.Log` はジョブのメインロジックをブロックしてはならない。

- **非同期書き込み**: `DBLogger` 内部で Go の channel を使い、バックグラウンドで一括書き込み（Batch Insert）を行う。
- **書き込み失敗時のフォールバック**: DB への書き込みに失敗した場合は、標準エラー出力にログを吐き出し、ジョブ自体の継続を優先する。

---

## 拡張性とメンテナンス

### 新規イベントの追加手順

1. `log-viewer` サブモジュールの `logger.go` に定数またはイベントキーを追加。
2. `log-viewer/ui/types.ts` に UI 側の型定義を追加。
3. 必要に応じて `JobLogViewer.tsx` に新しいイベントの表示ロジックを追加。
4. アプリケーションコードで `logger.Log(ctx, event)` を呼び出す。

### 互換性の維持

`detail_json` はスキーマレスだが、UI 側が壊れないよう以下のルールを守る。

- 既存のキーの型を変更しない。
- 必須項目がない場合は UI 側で適切にフォールバック（Optional Chaining 等）する。

---

## Go Context Management

Logger を各レイヤーに引き回すために、context に logger を保持するヘルパーを `log-viewer` パッケージに提供する。

```go
package joblog

type ctxKey struct{}

// WithLogger returns a context with the logger attached.
func WithLogger(ctx context.Context, l Logger) context.Context {
    return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the logger from the context, or a NoopLogger if not found.
func FromContext(ctx context.Context) Logger {
    if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
        return l
    }
    return &NoopLogger{}
}
```

Worker のジョブ実行エントリーポイント（`Process` メソッド）で `DBLogger` を生成し、context に注入する。これにより、配下の tool や service が明示的に logger を引数で受け取らなくても `Log` を呼べるようにする。

---

## UI Data Flow

`JobLogViewer` は React Query またはそれに類する hooks を使い、以下のフローでデータを取得する。

1. **初期ロード**: `ListJobLogs(jobId)` を呼び出し、過去のログを取得。
2. **ポーリング**: ジョブが `running` または `queued` 状態の間、3秒間隔で `ListJobLogs` を呼び出す。
3. **差分更新**: `timestamp` または `source_id` を使い、既存のログ一覧に新しいログを追記する。
4. **イベントフィルタ**: UI 側の state で表示を切り替える（API への再リクエストは不要だが、件数が多い場合は `SearchJobLogs` に切り替える）。

---

## 実装順序

1. **log-viewer サブモジュール** — `logger.go`, `noop.go`, `ui/JobLogViewer.tsx`
2. **DB migration** — `job_logs` テーブル
3. **shared** — `LogJobEvent` インターフェース + `DBLogger` 実装
4. **API read model** — `job_logs` と `job_mutation_logs` を unified event に正規化
5. **protobuf** — `ListJobLogs` RPC 追加 → buf generate
6. **API handler** — `ListJobLogs` / `SearchJobLogs` / `ListRelatedJobLogs` ハンドラー
7. **UI 移行** — `AgentTraceViewer` の Flow/List 機能を `log-viewer/ui` に移植
8. **Web 差し替え** — Audit ページを `JobLogViewer` import に変更
9. **統合** — 各 `log.Printf` を `logger.Log` に置き換え
10. **検索/関連ビュー** — Search UI と Related view を追加
11. **削除** — `AgentTraceViewer.tsx` と `listJobMutationLogs` の直接利用を削除

---

## 既存ログ実装の扱い

既存の `log.Printf` 呼び出しは、ステップ 9 で `logger.Log` に置き換える。
置き換えまでは暫定ログとして残してよい。

GCS へのジョブ単位アップロード案は採用しない。
ログの永続化と表示は `log-viewer` サブモジュールと DB-backed logger に寄せる。
