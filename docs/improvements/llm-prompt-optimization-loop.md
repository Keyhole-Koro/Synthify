# LLM Prompt Optimization Loop

## 背景

LLM Eval Runner は `knowledge_tree` の Tool eval、JSON report、Cloud Run Job、GCS artifact 保存まで進んだ。
次の課題は、eval 結果をもとに prompt 改善案を作り、同じ case set で安全に検証する loop を作ることである。

この loop では、LLM は本番 prompt を直接書き換えない。
LLM は失敗理由の分析と prompt variant の生成だけを担当し、採用は BI 上の Eval Review で人間が決める。

基盤設計は [llm-eval-runner.md](llm-eval-runner.md) を参照する。
実装済みコンポーネント契約は [llm-eval-runner-contract.md](../contracts/llm-eval-runner-contract.md) と [llm-eval-gcs-artifact-contract.md](../contracts/llm-eval-gcs-artifact-contract.md) を参照する。

### 現在地 (2026-05-18 時点)

「最初の実装単位」3 つ（Phase 1〜3）は **実装済み**。確定仕様は
[prompt-variant-eval-contract.md](../contracts/prompt-variant-eval-contract.md) に切り出した。
Phase 4 以降（Analyst LLM / Prompt Writer LLM / BI Eval Review）は未着手。

- Phase 1: prompt は [prompts](../../apps/worker/pkg/worker/prompts) package に `go:embed` で外出し済み。`prompts.Renderer` が render し、`process.GenerateKnowledgeTree` はそれを使う。hardcoded prompt は廃止。
- Phase 2: eval runner に `--variant` 実装済み。未指定時は production renderer、指定時は `apps/eval/variants/{name}`。不在 variant は exit `2`。
- Phase 3: `--golden` / `--update-golden` 実装済み。strict フィールド判定、`golden_diff` を report に additive 追加。
- 旧 `synthesis` 命名は廃止し、tool key は `knowledge_tree` に統一（命名規約は契約 §0）。

未着手は Phase 4 以降と、`apps/eval/analysis/` 系。この loop の自動化部分はまだ存在しない。

## 目的

固定 case / golden / GCS artifact を使って、prompt 改善の仮説作成から検証までを再現可能にする。

目標:

- eval report と golden diff から失敗理由を構造化する
- prompt variant を本番 prompt から分離して生成する
- baseline と variant を同じ case set で比較する
- 人間が採用判断できる report と rationale を残す

非目標:

- 本番 prompt の自動書き換え
- variant の自動採用
- 初期段階での Agent eval 対応
- すべての worker tool への一括対応

## Architecture

| Role | 責任 | 備考 |
| :--- | :--- | :--- |
| Generator LLM | `knowledge_tree` output を生成する | eval 対象。現行の Gemini model を使う |
| Analyst LLM | eval report / golden diff を読み、失敗理由と改善方針を構造化する | Generator と同じ model でもよいが、役割は分ける |
| Prompt Writer LLM | Analyst output をもとに prompt variant を生成する | 本番 prompt ではなく variant directory にだけ書く |
| Eval Runner | baseline / variant を同じ case set で実行し、rule / golden / usage を比較する | `apps/eval` に閉じる |
| BI | eval artifact、golden diff、variant 比較、review 状態を表示する | 旧 log viewer を BI として扱う |
| Human Reviewer | BI 上で prompt diff、rationale、eval report を確認し採用可否を決める | approve と apply は分ける |

全体フロー:

```text
production prompt
  ↓
baseline eval
  ↓
rule score + golden diff
  ↓
Analyst LLM: failure analysis
  ↓
Prompt Writer LLM: generated variant
  ↓
variant eval
  ↓
baseline vs variant report
  ↓
BI Eval Review: approve or reject
  ├─ reject → Analyst LLM に戻り再試行 (iteration 上限まで)
  └─ approve → Apply approved variant
```

Phase 6 までは人間が手動で回す one-shot pipeline であり、自動ループバックは初期非目標とする（[非目標](#目的) 参照）。上図の reject 矢印は「人間が再試行を選んだ場合」の経路であって、自動再投入ではない。

## Implementation Phases

### 1. `knowledge_tree` prompt 外出し

`apps/worker/pkg/worker/prompts` を作り、`go:embed` で本番 prompt を埋め込む。

```text
apps/worker/pkg/worker/prompts/
  prompts.go
  templates/
    knowledge_tree.system.tmpl
    knowledge_tree.user.tmpl
```

`process.GenerateKnowledgeTree` は hardcoded `SystemPrompt` / `UserPrompt` をやめ、prompt renderer から render した prompt を使う。
eval runner も同じ production prompt を使えるようにする。

**prompt の source of truth は repo file + `go:embed` で暫定確定する。** GCS prompt registry / 管理 DB 化は Phase 1 のアーキテクチャ（埋め込み vs ランタイム取得）を変えてしまうため、loop 全体が安定するまで採用しない。これは Open Questions ではなく Phase 1 着手の前提とする（後続で registry 化する場合も、renderer 抽象を挟んでおけば差し替えられる構成にする）。

### 2. Variant prompt 対応

eval runner に prompt renderer の差し替えを追加する。

CLI:

```bash
synthify-eval \
  --cases apps/eval/cases \
  --variant concise-structure-v1
```

variant 未指定時は production prompt を使う。

variant layout:

```text
apps/eval/variants/
  concise-structure-v1/
    knowledge_tree.system.tmpl
    knowledge_tree.user.tmpl
```

generated variant は本番 image に混入させない。Cloud Run Job の通常 eval は production prompt を使う。

### 3. Golden diff

case ごとの期待 output を保存する。

```text
apps/eval/golden/
  knowledge_tree_api_spec.json
  knowledge_tree_meeting_notes.json
  knowledge_tree_numeric_spec.json
```

CLI:

```bash
synthify-eval --cases apps/eval/cases --golden apps/eval/golden
synthify-eval --cases apps/eval/cases --update-golden apps/eval/golden
```

初期 strict 判定に含めるもの:

- item count
- title set
- parent structure (`local_id`, `parent_local_id`)
- `source_chunk_ids`
- max depth

初期 strict 判定に含めないもの:

- `content` HTML 全文
- description の表現差分
- item ordering の軽微な差分

golden mismatch は exit 1 にする。golden 更新は `--update-golden` 明示時のみ行う。

**初回 golden の作り方**: `--update-golden` で現在の production prompt 出力をそのままスナップショットするだけでは、改善前の出力を「正解」として固定してしまい改善ループの意味が薄れる。初回 golden は必ず人手レビューを通す。運用は「`--update-golden` で雛形生成 → 人間が strict 判定対象フィールド（item count / title set / parent structure / `source_chunk_ids` / max depth）を確認・修正して commit」とする。以降の `--update-golden` も無条件採用ではなく、diff を人間が確認したうえで commit する前提とする。

### 4. Analyst LLM

eval report と golden diff を入力に、失敗理由を構造化出力する。

出力例:

```json
{
  "summary": "Required source headings are being paraphrased too aggressively.",
  "failure_patterns": [
    {
      "case_name": "knowledge_tree_api_spec",
      "issue": "required title missing",
      "likely_prompt_cause": "prompt does not emphasize preserving source section names"
    }
  ],
  "recommendations": [
    "Preserve source headings as candidate item titles when they represent core concepts."
  ]
}
```

CLI は将来 subcommand として追加する。

```bash
synthify-eval analyze \
  --report gs://.../eval/stage/runs/2026/05/17/run.json \
  --out apps/eval/analysis/latest.json
```

### 5. Prompt Writer LLM

Analyst output と production prompt を入力に、prompt variant を生成する。

出力先:

```text
apps/eval/variants/generated/{timestamp}/
  knowledge_tree.system.tmpl
  knowledge_tree.user.tmpl
  rationale.md
```

`rationale.md` には、どの失敗を改善しようとしたか、どの制約を強めたかを記録する。
Prompt Writer LLM は production prompt を直接変更しない。

CLI:

```bash
synthify-eval write-prompt \
  --analysis apps/eval/analysis/latest.json \
  --out apps/eval/variants/generated/20260517T042000Z
```

### 6. Baseline vs Variant 比較

同じ case set / golden に対して baseline と variant を実行し、比較 report を出す。

比較項目:

- pass count
- golden match count
- missing titles
- item count / max depth
- token usage
- duration
- Analyst recommendation に対する改善有無

variant が baseline より良い場合でも自動採用しない。
Human Reviewer が BI 上で prompt diff、rationale、eval report、golden diff を確認して approve / reject する。

### 6.1 ループの終了条件

「Optimization Loop」と呼ぶ以上、いつ止めるかを設計に含める。

- **iteration 上限**: 1 つの failure analysis に対して Prompt Writer が生成する variant は最大 N 回（初期 N=3）。N に達しても baseline を超えなければ、その分析は人間にエスカレーションして打ち切る。
- **勝てない場合の打ち切り**: variant が baseline を pass count / golden match で下回り続けた場合、自動で再投入せず人間が継続可否を決める。
- **golden 不正の検知**: variant・baseline ともに同じ case で golden mismatch する場合、prompt ではなく golden 自体が誤っている可能性を疑う経路を用意する（その case を golden 要再レビューとしてマークし、ループから一時除外する）。
- **コスト打ち切り**: 後述のコスト上限（Guardrails 参照）に達したら、結果にかかわらず iteration を停止する。

### 7. BI Eval Review

BI に eval artifact を確認する review 画面を追加する。
この画面は、評価結果を見るだけでなく prompt variant の採用判断も扱う。

BI で表示するもの:

- eval run 一覧
- GCS artifact URI
- baseline / variant の pass count
- case ごとの schema / rule / golden diff
- prompt diff
- Analyst rationale
- token usage / duration / model
- review status

review status:

```text
pending
approved
rejected
applied
superseded
```

`approve` は「この variant を採用してよい」という判断だけを記録する。
`apply` は approved variant を production prompt に反映する別操作にする。
誤操作で即本番 prompt が変わらないよう、approve と apply は同一ボタンにしない。

初期実装では、既存の `job_approval_requests` は流用しない。
job approval は job execution plan の承認に寄っているため、prompt variant 採用に混ぜると意味が曖昧になる。
Eval Review は専用の小さな model / API として追加する。

最小 model:

```text
prompt_variant_reviews
- review_id
- run_artifact_uri
- baseline_artifact_uri
- tool
- case_name or suite_name
- variant_name
- status
- reason
- requested_by
- reviewed_by
- created_at
- reviewed_at
- applied_at
```

反映処理は、approved review だけを対象にする。
production prompt への反映方式は Phase 1 と同じく **repo file 更新**（`apps/worker/pkg/worker/prompts/templates/` への commit）で暫定確定する。GCS prompt registry 化は loop 安定後の後続課題とする。

## Storage Layout

```text
apps/worker/pkg/worker/prompts/templates/
  knowledge_tree.system.tmpl
  knowledge_tree.user.tmpl

apps/eval/variants/
  {variant_name}/
    knowledge_tree.system.tmpl
    knowledge_tree.user.tmpl

apps/eval/variants/generated/
  {timestamp}/
    knowledge_tree.system.tmpl
    knowledge_tree.user.tmpl
    rationale.md

apps/eval/golden/
  {case_name}.json

apps/eval/analysis/
  latest.json
```

GCS artifact は eval report の長期保存に使う。
generated variants を GCS artifact として残すか repo commit 対象にするかは未決定とする。

## Guardrails

- LLM は production prompt を直接変更しない。
- 本番 prompt 反映は BI 上の人間 review と明示的な apply 操作を必須にする。
- approve と apply を同一操作にしない。
- 既存の job approval model を prompt variant 採用に流用しない。
- generated variant は本番 worker image に混入させない。
- `content` HTML 全文は初期 golden diff の strict 判定に含めない。
- golden 更新は `--update-golden` 明示時のみ行う。
- baseline より token usage が大きく増えた variant は、品質が上がっていても採用時に人間が確認する。
- Analyst / Prompt Writer の出力は補助情報であり、採用判断の単独根拠にしない。
- **loop 自体のコスト上限を設ける。** 1 回の最適化サイクルは Analyst + Prompt Writer + baseline eval + variant eval × iteration を LLM で回すため、1 サイクルあたりの LLM 呼び出し回数・推定トークンに上限を設定し、上限到達でループを停止する。閾値は [usage-based-billing.md](usage-based-billing.md) の予算アラート方針と整合させる。Cloud Run Job で回す場合も同じ上限を適用する。

## Open Questions

Phase 1（最初の実装単位）着手前に決める必要があるもの:

- prompt 外出しを `knowledge_tree` だけ先行するか、`summary` / `briefing` / `critique` / `merging` も同時に進めるか。
- golden mismatch をすべて fail にするか、warn mode を用意するか（Phase 3 の exit code を規定する）。

（解決済み: prompt の source of truth と apply 先 → repo file + `go:embed` で暫定確定。背景・Phase 1・Phase 7 参照。）

後続フェーズで決めればよいもの:

- Analyst LLM / Prompt Writer LLM の model を Generator と同一にするか、別 model にするか。
- generated variants を repo commit 対象にするか、GCS artifact として残すか。
- variant 勝敗の採用条件をどこまで機械判定するか。
- apply 済み prompt の rollback を BI から扱うか。

## 最初の実装単位

最初は次の 3 つに絞る。

1. `knowledge_tree` prompt 外出し
2. eval runner の `--variant`
3. golden diff

この 3 つの確定仕様は [prompt-variant-eval-contract.md](../contracts/prompt-variant-eval-contract.md) に切り出した。実装はこの契約に従う。

Analyst LLM / Prompt Writer LLM は、baseline / variant / golden の評価基盤が安定してから追加する。
