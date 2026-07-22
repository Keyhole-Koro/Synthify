# Eval トレース: 並列 / エージェント的 multi-LLM 実行への対応

## 背景

eval のトレース収集（`apps/eval/runner` の `TraceCollector`、PR #20 で導入）は、
1 case の実行を `tool → llm` の親子ツリー＋ `validation` / `assertion` として
`eval_trace_events`（`parent_event_id` / `sequence`）に記録する。

現在の親子付けは **単一のスパンスタック**方式で、「今開いている一番内側のスパン = 親」
とみなす。これは **逐次ネスト実行を前提**にしている。

現状はこの前提で正しい:

- eval の `knowledge_tree` tool は LLM を 1 回だけ呼ぶ（`GenerateStructured`。失敗時の
  再パース fallback は追加の LLM 呼び出しではない）。
- worker の process tool 群も逐次・単一呼び出しで、tool 層に `errgroup` / goroutine による
  LLM の並列 fan-out は無い。
- ADK エージェントランタイム（sub-agent の並列実行など）は eval 経路では使っていない。

したがって現時点でバグは無い。本チケットは **将来の並列化に備えた設計負債の記録**である。

## 問題

「1 つの処理が 2 つ以上の LLM を並列に呼ぶ」「1 つの LLM/エージェントが複数の
sub-tool を並列に呼ぶ」構造が入ると、現在の stack モデルは **論理的に破綻する**:

- `mutex` があるのでデータ競合（クラッシュ）は起きない。
- しかし複数スパンが同時に open すると「スタックの一番上」が曖昧になり、
  子 LLM が **誤った親にぶら下がる**。`sequence` も実時間順と親子順が混ざる。
- 結果として「並列兄弟（同じ親に複数の子）」を正しく表現できない。

## 方針

ambient なスタックをやめ、**明示的な parent span ID を `context` で伝播**する方式に置き換える。

- `context.WithValue` で現在の span ID を `ctx` に積む。
- 各 goroutine は親の `ctx` を受け取り、自分のスパンをその親 ID に紐付ける。
- `TraceCollector` から `stack` を削除し、`Record` は呼び出し側が `ctx` から取得した
  `ParentEventID` を受け取る（`BeginSpan` は開始した span 入りの `ctx` を返す）。

ストレージと UI は変更不要な見込み:

- DB スキーマ（`parent_event_id` + `sequence`）は既に DAG / 並列兄弟を表現できる。
- `EvalExecutionTrace` は `parent_event_id` から depth を計算しているため、
  同じ親に複数の子があってもそのままツリー描画できる。

つまり **収集側（collector + runner の span 開閉）に閉じた変更**で済む。

## スコープ外

- 実際に LLM を並列に呼ぶ tool / エージェントの実装そのもの（これが入る PR で本対応を行う）。
- span ID の分散トレーシング標準（OpenTelemetry など）への準拠。まずは既存 `eval_trace_events`
  スキーマの範囲で完結させる。

## 完了条件

- [ ] `TraceCollector` が `context` 伝播ベースの親付けに移行し、`stack` を廃止している。
- [ ] 同一 case 内で複数スパンが並列に open しても、各子スパンの `parent_event_id` が
      正しい親を指す（並列を模したユニットテストで検証）。
- [ ] 既存の逐次トレース（`tool → llm` + `validation` / `assertion`）の記録内容と
      `TestRunCase_CollectsTrace` の期待値が回帰しない。
- [ ] Monitor の `EvalExecutionTrace` が「同じ親を持つ並列兄弟」を正しくツリー表示できる
      ことをスクリーンショット fixture で確認する。

## 実施タイミング

agentic / 並列 multi-LLM を実際に入れる設計（どの tool が何を並列に呼ぶか）が固まった段階で、
その PR に本対応を含める（YAGNI。並列実行が存在しない今は先行実装しない）。

## 参考

- 収集実装: `apps/eval/runner/trace.go`（`TraceCollector` / `BeginSpan` / `EndSpan` / `Record`）
- 記録箇所: `apps/eval/runner/runner.go` `runCase`（tool span の開閉）
- スキーマ: `db/migrations/0023_eval_trace_events`、view `v_eval_trace_events`
- UI: `apps/monitor/ui/src/components/eval/EvalExecutionTrace.tsx`（`parent_event_id` から depth 計算）
- 関連 PR: #18（eval 永続化）→ #19（トレースビューア UI）→ #20（実トレース計測 + ネスト span）
