# Worker agent ループの暴走と timeout 固着

> **Status: L1 / L2 / L3 / L4-a / L4-b 実装済み（2026-06-01）。**
> L1=Document Map 自動注入、L2=wall-clock budget、L3=env var 一本化の timeout 整合、
> L4-a=`context.WithoutCancel` で必ず FAILED、L4-b=per-item checkpoint +
> RETRYABLE 状態での自動再開（[worker-budget-retry-state.md](worker-budget-retry-state.md)）。
> persist 冪等化（L4-c の前提）のみ未着手
> （[persist-knowledge-tree-idempotency.md](persist-knowledge-tree-idempotency.md)）。

## TL;DR

2026-06-01、stage 環境で 3 chunk しかない小さな Mermaid ドキュメントの処理が
**300 秒 timeout で失敗し、しかも job が `RUNNING`（"Processing started"）のまま
永久に固着した**。

ログ実測で判明した真因は「巨大ドキュメントだから遅い」ではない。**agent が
`extract_text` の正しい呼び出し方を知らず、当て推量で 10 回以上ツールを叩いて
~3 分を浪費した**こと。そしてその迷子を止める仕組みも、timeout 後に job を
確実に FAILED へ落とす仕組みも無かった。

問題は独立した 4 層に分かれており、どれか 1 つを直すだけでは再発する。本ドキュメントは
4 層すべてを記録し、解決策を層ごとに検討する。

---

## 観測された事実（2026-06-01, synthify-stage）

対象 job: `01KT1G0TMS8DESZDCDR603BRCE`
対象 document: `01KT1G0Q4V6Z015HK4BSTHCJWB`（Mermaid 解説、chunk 3 個）

### タイムライン

```
11:45:17  最初の extract_text                 ← ここから探索フェーズ
  ...     extract_text / grep_search を15回   ← 当て推量の総当たり（後述）
11:48:15  extract_text "."                     ← 約3分を探索に浪費
11:48:23  generate_brief                       ← ようやく本処理開始
11:49:04  analyze_dependencies
11:49:20  generate_knowledge_tree
11:49:40  generate_html_summary (1/N)
11:49:52  generate_html_summary (2/N)
11:50:08  generate_html_summary (3/N)          ← 最後のLLM応答
11:50:04  HTTP 504 / latency=300.000s          ← Cloud Run timeout 到達
11:50:12  context canceled の連鎖（後述）
11:50:42  Cloud Tasks リトライ → skip_running_job (no-op)
```

合計 **37 回の LLM ラウンドトリップを直列実行**。各往復 3〜35 秒（中央値 ~6 秒）。

### 探索フェーズで agent が実際に投げた引数

```
extract_text  01KT1G0Q4V6Z...            ← document_id
extract_text  ['https://storage.../...]  ← フルURL
extract_text  01KT...HCJWB_extracted     ← "_extracted" を捏造
extract_text  01KT...HCJWB.png           ← ".png" を捏造
extract_text  ['']                        ← 空文字
extract_text  01KT...HCJWB/something      ← "something" 完全な当て推量
extract_text  01KT...HCJWB.json          ← ".json" を捏造
extract_text  .                           ← カレントディレクトリ
grep_search   .*                          ← 全マッチ（中身が見えていない兆候）
grep_search   the                         ← 英語最頻出語（無意味な総当たり）
```

`extract_text` の正しい引数は **`file_uri`**（[orchestrator.go の起動プロンプトで
agent に渡している](../../apps/worker/pkg/worker/agents/orchestrator.go)）だが、agent は
それを使わず `document_id` や捏造パスを延々と試した。

### context canceled の連鎖（固着の直接原因）

```
11:50:12  worker.internal_dispatch.execute_failed   err="agent run: context canceled"
11:50:12  repository.mark_job_failed_failed          err="fail job: context canceled"
11:50:12  jobstatus.firestore_write_failed           err="code = Canceled"
11:50:12  jobstatus.notify_failure_failed            err="code = Canceled"
```

Cloud Run が timeout でリクエスト ctx をキャンセル → worker は**同じ ctx**で
failJob しようとして巻き添えキャンセル → job が `RUNNING` のまま残る →
Cloud Tasks リトライは `shouldSkipJobStatus` が `RUNNING` を見て skip →
**永久固着**。

### GCS FUSE は無実

同時刻の gcsfuse ログは GC 52ms、遅延ログゼロ。時間の正体は I/O でも CPU でもなく、
**外部 LLM API 待ちの累積**。

---

## 問題の 4 層構造

```
[L1] プロンプト/ツール設計   agent が extract_text の正しい一手を知らず
                              当て推量を10回以上 → ~3分浪費          ← 真因
        ↓ これが無ければ全体2分で完了し、以下は発火しない
[L2] ループ予算              反復・時間の上限が無く、迷子を止められない
        ↓
[L3] timeout 設定            Cloud Run 300s（デフォルト放置）/
                              Cloud Tasks dispatch 600s に到達
        ↓
[L4] 失敗時の固着            キャンセル済み ctx で failJob も失敗 →
                              RUNNING 固着 → リトライも skip
```

### L1 — agent が `extract_text` で迷子になる（真因）

**事実:**
- `extract_text` の引数は `file_uri` / `document_id` / `mime_type` / `workspace_id` を
  すべて optional で受ける緩い設計
  （[extraction.go](../../apps/worker/pkg/worker/tools/builtin/io/extraction.go)）
- Description は「Extracts raw text from a given document URI」とだけ。
  **「最初の本文取得は、プロンプトで渡された `file_uri` をそのまま1回渡せ」とは書いていない**
- この病は既知。[extraction.go の内部コメント](../../apps/worker/pkg/worker/tools/builtin/io/extraction.go)に
  `// noise — the agent then re-calls extract_text in a loop trying to get usable`
  と、まさに今回の現象が記録されている

**なぜ起きるか:** agent には「本文をどう取り出すか」の確定的な入口が無く、
ツールスキーマの曖昧さと相まって、LLM が引数を試行錯誤する自由度が残っている。

### L2 — ループに時間・反復の予算が無い

**事実:**
- [orchestrator.go の `jobRunner.Run`](../../apps/worker/pkg/worker/agents/orchestrator.go) は
  `IsFinalResponse()` まで無制限に回る。`agent.RunConfig{}` は空で
  **反復回数上限も時間予算も無い**
- 唯一の歯止めは
  [repeat_guard](../../apps/worker/pkg/worker/agents/repeat_guard.go)（同一ツール連打検知）
  だが、今回のように**毎回違う引数**だと作動しない
  （`extract_text X` と `extract_text Y` は別物と判定される）

### L3 — timeout は意図でなくデフォルトの放置

**事実:**
- Cloud Run の `timeout` は
  [cloud_run_service モジュールのデフォルト `300s`](../../terraform/modules/cloud_run_service/variables.tf)。
  **worker サービスは上書きしていない**（[services/worker](../../terraform/services/worker/)）
- Cloud Tasks queue は `dispatch_deadline` を**明示していない** →
  デフォルト 600s。Cloud Run より長いので、Cloud Run 300s が先に効く
- Cloud Run の上限は 3600s、Cloud Tasks dispatch_deadline の上限は 1800s

### L4 — timeout 後に job を FAILED へ落とせず固着

**事実:**
- [worker.go の failJob](../../apps/worker/pkg/worker/worker.go) も、その先の
  Firestore 通知も、**リクエスト ctx を使う**。timeout でその ctx が
  キャンセル済みなので失敗処理ごと失敗する
- さらに `Process` には `MarkRunning` 後に **failJob を呼ばずに return する
  エラーパスが 3 つ**ある（completion 時の
  `GetDocumentRootItemID` / `GetWorkspaceRootItemID` / `lifecycle.Complete`）。
  これは prod でも 2026-05-30 に観測された固着の原因
- [shouldSkipJobStatus](../../apps/worker/pkg/worker/worker.go) が `RUNNING` を
  skip するため、固着した job はリトライでも前進しない

---

## 解決策の検討

要件は **「速く、かつ確実」**（インタラクティブに近い UX を保ちつつ、何があっても
固着させない）。各層に解決策を割り当てる。

### L1 への対策 — 探索ループを構造的に消す【最優先・速くなる】

迷子を確率的に減らすのではなく、**発生し得ない構造**にするのが「速く＋確実」に効く。

- **L1-a（プロンプト主義）**: `extract_text` の Description と orchestrator instruction を
  改善し、「初回抽出は渡された `file_uri` をそのまま使え」と明示。引数も
  `file_uri` 必須に絞る。低コスト・既存構造維持。ただし LLM 依存で**保証は無い**
  （別ドキュメントで別の迷い方をするリスク）
- **L1-b（決定論主義 / 推奨）**: 最初の本文抽出を agent ループから外し、
  `ProcessDocument` 冒頭で orchestrator が `extract_text(file_uri)` を**確定的に1回実行** →
  結果を「本文はこれ」と agent に渡す。探索ループ自体が構造的に発生しなくなる。
  中コスト・確実

> **論点:** chunk → tree → summary → persist も決定論パイプラインに寄せるか
> （= router-job-splitting の縮小版・パイプライン化）、それとも**抽出だけ確定化して
> 知的部分は agent に残す**か。前者は「速く＋確実」を最大化するが、worker の
> 「LLM がオーケストレーター」という[基本思想](../worker/llm-worker-architecture.md)からの
> 転換になる。

### L2 への対策 — ループに予算を入れる【保険・確実になる】

- 反復回数上限（例: N ターン）と、ジョブ全体の wall-clock 予算（例: timeout の 8 割）を
  `RunConfig` か orchestrator 側のループで強制
- repeat_guard を**引数の正規化後**に重複判定するよう改修
  （`extract_text` の捏造パス連打を「実質同じ無駄打ち」として検知）
- 予算超過時は「打ち切って FAILED」ではなく、可能なら**部分結果で persist** できるよう
  checkpoint と連携（後述）

### L3 への対策 — deadline を現実に合わせる【補助】

- worker の Cloud Run `timeout` を 300 → 900s 程度に上げる（services/worker で明示）
- Cloud Tasks の `dispatch_deadline` を **タスク単位**で設定し、Cloud Run timeout と
  整合させる（dispatch_deadline ≤ 1800s が上限。Cloud Run timeout > dispatch_deadline に
  すると二重ディスパッチで古いリクエストがキャンセルされるので、**dispatch_deadline ≥
  Cloud Run timeout** になるよう設計）。
  注: キューリソースには `dispatch_deadline` が無いので enqueue 時にタスクへ設定する
- ただし L1 が効けば主役ではない。**青天井の入力には延長では追いつかない**

### L4 への対策 — 何があっても FAILED に落とす【必須・固着を消す】

- failJob / Firestore 通知を `context.WithoutCancel(ctx)` + 短い専用 timeout で実行し、
  リクエスト ctx がキャンセル済みでも必ず status を遷移させる
- `Process` の `MarkRunning` 後の全エラーパスを failJob 経由に統一
  （completion 時の 3 つの return も含む）
- `shouldSkipJobStatus` の `RUNNING` skip を見直す。タイムアウトで死んだ job を
  リトライで**安全に再開**できるようにする（checkpoint と連携）か、
  少なくとも「最後の更新から N 分以上 RUNNING のままなら stale とみなして再実行可」に

---

## 既存設計との関係

- **[router-job-splitting.md](router-job-splitting.md)**: 「巨大ドキュメントを分割」。
  本問題は**小さなドキュメントでも起きる**ので別軸。ただし L1-b でパイプライン化する場合、
  Router 設計と統合し得る
- **[job-checkpoint-spec](../architecture/job-checkpoint-spec.md)（Done）**: checkpoint 機構は
  既に存在し動作している（[callbacks.go](../../apps/worker/pkg/worker/agents/callbacks.go)）。
  ただし checkpoint 対象は `generate_brief` / `generate_knowledge_tree` /
  `persist_knowledge_tree` の **3 ツールだけ**。時間を食う探索フェーズと
  `generate_html_summary` は checkpoint されておらず、リトライしても救われない。
  **checkpoint 対象の拡張**が L2/L4 と連動する
- **[capability-limits-not-enforced.md](capability-limits-not-enforced.md)**: LLM 呼び出し
  上限が強制されていない問題と L2 は同根。統合して扱える
- **[agent-error-silenced.md](agent-error-silenced.md)**: agent エラーの握りつぶしと L4 は関連
- **[worker アーキテクチャ思想](../worker/llm-worker-architecture.md)**: 「LLM が
  オーケストレーター、コードは起動だけ」。L1-b でパイプライン化する範囲は、この思想を
  どこまで保つかの判断になる

---

## 推奨アプローチ（2026-06-01 議論で合意）

### 着手順序: L4 → L1 → L2 → L3

**「被害の止血」と「真因の根治」を分ける。** 根治（L1）は設計判断と実装に時間が
かかるため、その間も timeout 由来の固着が続く。先に L4 で「最悪でも FAILED として
返る」状態を作り、安心して L1 を進める。各ステップは独立して価値を出す
（L4 だけでも固着は消える）。

#### 1. L4 — 固着を止める + persist 手前までの再開【最優先】

**L4-a 止血（必須）:**

- `failJob` と Firestore 通知を `context.WithoutCancel(ctx)` + 短い専用 timeout で
  実行し、リクエスト ctx がキャンセル済みでも必ず status を遷移させる
- `Process` の `MarkRunning` 後の全エラーパス（completion 時の
  `GetDocumentRootItemID` / `GetWorkspaceRootItemID` / `lifecycle.Complete` の
  3 つの return を含む）を failJob 経由に統一
- これだけ入れれば "Processing started" の永久固着は消え、ユーザーには
  少なくとも失敗が返る

**L4-b 軽い自動再開（persist 手前に限定）:**

- 再開で危険なのは **persist だけ**。`persist_knowledge_tree` は item ごとに
  個別 tx で `Create...` し、`document_id` に UNIQUE 制約があるため、
  **二度実行すると重複作成 or 制約違反になる**（冪等でない。
  [item.go](../../apps/worker/pkg/worker/repository/postgres/item.go)）。
  persist 全体は 1 tx でもないので「item を数個作って途中で死ぬ」状態が起こりうる
- それ以外（抽出・brief・tree 生成）は冪等 or 再実行安全。brief / knowledge_tree は
  既に checkpoint される
- 方針: **persist が一度でも始まった job は自動再開しない**（FAILED 確定 → 手動再投稿）。
  persist より手前で死んだ job だけ、既存 checkpoint（brief / tree）を使って
  **重い前半（探索 3 分）をスキップして再開**できる
- これにより persist の冪等化に触れずに「探索やり直し」の最悪ケースだけ救える

**L4-c フル自動再開（今回やらない・別タスク）:**

- persist 途中で死んだ job の自動再開は、persist の tx 化 + 冪等化が前提。
  [persist-knowledge-tree-idempotency.md](persist-knowledge-tree-idempotency.md) と
  [resume-processing-stub.md](resume-processing-stub.md) に切り出し

#### 2. L1 — 本文を Document Map として自動注入【真因の根治・速くなる】

採用は **L1-b（決定論）**。ただし射程は「生全文の注入」ではなく、
**Working Memory 原則の一貫適用**とする。

- `ProcessDocument` 冒頭で、コードが確定的に 1 回だけ
  `extract_text(file_uri)` → `BuildChunks` を実行
  （[chunking.go](../../apps/worker/pkg/worker/document/chunking.go) は既に
  `maxRunes=3500` で分割し見出しも持つ。データは揃っている）
- chunk の **目次（見出し + chunk_id）だけ**を 4 つ目の Working Memory ブロック
  「**Document Map**」として自動注入する。**生全文は載せない**
  （大きいドキュメントでコンテキストを潰さないため）
- これにより agent は起動時から「文書に何があるか」が見え、`extract_text` を
  そもそも呼ぶ必要がなくなる → 探索ループが構造的に消える
- 既存の Brief / Glossary / Journal と同じ
  [自動注入の仕組み](../../apps/worker/pkg/worker/agents/callbacks.go)に乗せる

> 射程は「抽出 + chunk + 目次注入」までの確定化に留める。chunk→tree→summary→persist の
> 知的部分は agent に残す（「LLM がオーケストレーター」思想を維持）。
> パイプライン全面化は、まず本対策の効果を測ってから別途判断。

#### 3. L2 — ループ予算を配線【保険】

- 新規設計ではなく**既存の受け皿を配線するだけ**。
  [JobCapability に `max_llm_calls` / `max_tool_runs` が既に定義済み](../worker/llm-worker-capability-spec.md)で、
  [強制されていないだけ](capability-limits-not-enforced.md)。
  本件と capability-limits-not-enforced は統合して扱う
- repeat_guard を**引数の正規化後**に重複判定するよう改修
  （捏造パス連打を「実質同じ無駄打ち」として検知できるように）

#### 4. L3 — deadline を整合【補助・最後】

- L1 が効けば主役ではない。最後に Cloud Run `timeout`（services/worker で明示）と
  Cloud Tasks `dispatch_deadline` の整合を取る
  （**dispatch_deadline ≥ Cloud Run timeout** にして二重ディスパッチを防ぐ）

### Document Map に載せる内容: 見出し + chunk_id（全文なし）

```
この文書は3つの chunk に分かれています:
  chunk_0: Introduction to Mermaid
  chunk_1: Flowchart Syntax
  chunk_2: Advanced Diagram Types
```

agent は必要な chunk だけを id で引く（chunk 本体取得は既存の chunk 参照ツールが担う）。

## 決着済みの細部（2026-06-01）

- **L4 の RUNNING 再開:** persist 手前までの自動再開（L4-b）。persist 以降は止血のみ。
  フル自動再開（persist 冪等化が前提）は別タスク
  （[persist-knowledge-tree-idempotency.md](persist-knowledge-tree-idempotency.md)）
- **Document Map の粒度:** **見出し + 各 chunk の冒頭 1〜2 行**。
  弱い見出し（"Section 1" 等）の文書でも agent が往復なしで判断できるように。
  100 chunk でも 5〜10KB に収まる
- **L2 予算の強制点:** **実装時に前提が覆った。** 調査の結果、`max_llm_calls`
  enforcement は**既に完全実装・稼働中**だった
  （[usage.go](../../apps/worker/pkg/worker/tools/core/base/usage.go) の `increment` が
  上限超過で `reportExceeded` を返し、`IncrementLLMCalls` 経由でループを止める。
  `MaxLLMCalls=128` が [api/domain/job.go](../../apps/api/internal/domain/job.go) で
  発行済み）。`capability-limits-not-enforced.md` も実体が無い（README のリンク切れ）。
  そして **128 回上限は今回の問題に効かない** — timeout job は 37 往復で、回数ではなく
  **wall-clock 時間**が制約だった。repeat_guard も「同 tool + 同 args」連続のみ検知する
  ため、毎回 args が違う探索ループ（`extract_text(document_id)` →
  `extract_text(".png")` → …）はすり抜けた。
  → **L2 で足したのは「時間予算」**（下記実装ノート参照）

## 実装ノート（2026-06-01 着手分）

L4-a / L1 / L2 / L3 を実装。ポイントと、設計から変わった点:

### env var 一本化（L2 + L3 を統合）

タイムアウト系の値が複数箇所でズレる事故を防ぐため、**`request_timeout_seconds` を
terraform の単一の変数**にし、そこから:

- Cloud Run の `timeout`（ハードキャンセル境界）
- 環境変数 `WORKER_REQUEST_TIMEOUT_SECONDS`（worker が読む）

を**両方導出**する（[services/worker](../../terraform/services/worker/)）。worker は
起動時にこの env を読み（[config.go](../../apps/worker/pkg/worker/config/config.go)）、
`AgentBudget() = RequestTimeout × 0.9` を計算。`Process` が `ProcessDocument` を
この budget の `context.WithTimeout` でラップするので、worker は **Cloud Run の
ハードキャンセルより先に自分で打ち切り → failJob（親 ctx は生存中なので必ず FAILED に）**。

値の階層（**内 < 中 ≤ 外**）:

値の階層（**内 < 中 ≤ 外**）:

| 層 | 値 | 役割 |
|---|---|---|
| worker の AgentBudget | 540s（=600×0.9） | 一番内。worker が自分で打ち切り FAILED |
| Cloud Run timeout | 600s（10 分） | その外。worker が自打ちできなかった時のハードキャンセル |
| Cloud Tasks task DispatchDeadline | 900s（15 分） | 一番外。これより短いとリトライ二重実行 |

Cloud Run 既定の 300s は「意図でなくモジュールのデフォルト放置」だったため、
worker module に `timeout` を明示して 600s に。

`dispatch_deadline` は **キューリソースではなくタスク単位**で設定する
（google provider 6.x の `google_cloud_tasks_queue` には `dispatch_deadline`
引数が存在しない）。API が enqueue する際に `taskspb.Task.DispatchDeadline` を
15m に設定（[cloudtasks_dispatcher.go](../../apps/api/internal/infrastructure/worker/cloudtasks_dispatcher.go)）。
これで未設定時の API 既定 600s を上回り、Cloud Run 600s と並ぶ事故を避ける。
