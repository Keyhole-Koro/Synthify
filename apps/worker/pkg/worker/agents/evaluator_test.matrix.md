# テストマトリクス: `evaluator_test.go`

このマトリクスは、`evaluator_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

> **読み方の注意**: 下の「テストケース表」は既存テストが確認している内容を写したものです。
> テストがない分岐は表に現れないため、「インターフェース網羅チェック」「依存エラー軸」「未テスト分岐 (GAP)」も併読してください。
> カバレッジ数値は未計測 (2026-07-18)。実測時は `go test -coverprofile` の値へ更新すること。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

## インターフェース網羅チェック (`Evaluator`)

| メソッド / 経路 | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `NewEvaluator` | ◐ 各テストの setup で使用 | 未計測 | PARTIAL | constructor 単体の確認はないが、状態を持たないため優先度は低い。 |
| `EvaluateTree`: 正常な構造化出力 | ✅ 3 subtests | 未計測 | OK | summary/findings の内容品質、空文字列の扱い。 |
| `EvaluateTree`: score→passed の決定論的判定 | ✅ 3 subtests | 未計測 | OK | 閾値変更時の設定化。 |
| `EvaluateTree`: score 範囲 validation | ✅ 2 subtests | 未計測 | OK | 極端な整数、型違い、score 欠落。 |
| `EvaluateTree`: JSON parse error | ✅ 1件 | 未計測 | OK | schema 上の必須フィールド欠落、未知フィールド。 |
| `EvaluateTree`: LLM dependency error | ✅ 1件 | 未計測 | OK | context cancellation / deadline、rate limit の型付き error。 |
| `EvaluateTree`: nil LLM | ✅ 1件 | 未計測 | OK | nil を error とするか結果として返すかの仕様変更。 |

## 依存エラー軸 (dependency returns error)

`Evaluator` の外部依存は `llm.Client.GenerateStructured`。☑=テスト有 / ◐=間接 / ❌=未テスト。

| メソッド | provider error | malformed response | context canceled | deadline exceeded | rate limit / retryable error |
| --- | --- | --- | --- | --- | --- |
| `EvaluateTree` | ☑ `TestEvaluateTree_WrapsLLMError` | ☑ `TestEvaluateTree_RejectsMalformedJSON` | ❌ | ❌ | ❌ |

→ Provider error のラップは確認済みだが、呼び出しキャンセルと provider error classification は未確認。

## 未テスト分岐 (GAP) — テストケース表に現れない経路

| 場所 | 分岐 | 期待挙動 | なぜ重要か |
| --- | --- | --- | --- |
| `evaluator.go:EvaluateTree` | `score` フィールド欠落 | 欠落を schema error として拒否するか、0点として扱うかを仕様化する。 | 現状は Go の zero value により静かに0点となる。Providerのschema保証が崩れた際に検知しにくい。 |
| `evaluator.go:EvaluateTree` | `summary` / `findings` 欠落または空 | 必須性と最小品質を検証する。 | scoreだけの評価ではレビュー理由が残らない。 |
| `evaluator.go:EvaluateTree` | context canceled / deadline exceeded | 元errorを保持して速やかに返す。 | Worker停止・予算超過時に評価処理が残留するのを防ぐ。 |
| `evaluator.go:EvaluateTree` | 巨大な `treeData` | 入力上限またはchunking方針を適用する。 | token超過、遅延、コスト増加の防止。 |
| `evaluator.go:EvaluateTree` | findings が極端に多い / 長い | 件数・長さ上限を適用する。 | DB・ログ・UIへの過大データ流入を防ぐ。 |

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestEvaluateTree_DerivesPassedFromScore/below_threshold_overrides_true` | `EvaluateTree` | 矛盾出力 / 閾値 | model が `score=69, passed=true` を返す。 | `Passed=false`。 | なし。 | `out.Passed == false`, `out.Score == 69` | model booleanを信用せずscoreを正とすること。 | score欠落、型違い。 | `score` missing / string のschema error。 | OK |
| [ ] | `TestEvaluateTree_DerivesPassedFromScore/threshold_overrides_false` | `EvaluateTree` | 境界値 | model が `score=70, passed=false` を返す。 | `Passed=true`。 | なし。 | `out.Passed == true`, `out.Score == 70` | 合格閾値ちょうど70を許可すること。 | 71や100の境界。 | 100点と0点の明示テスト。 | OK |
| [ ] | `TestEvaluateTree_DerivesPassedFromScore/above_threshold_passes` | `EvaluateTree` | 正常系 | model が `score=95, passed=true` を返す。 | scoreと合否を保持する。 | なし。 | `Passed`, `Score`, prompt, schema type | 正常な高評価、tree入力のprompt注入、schema指定。 | summary/findingsの内容品質。 | summary/findingsの保持assertion。 | PARTIAL |
| [ ] | `TestEvaluateTree_RejectsOutOfRangeScore/score_-1` | `EvaluateTree` | 数値下限 | `score=-1`。 | range error。 | なし。 | error contains `must be between 0 and 100` | 負数を拒否すること。 | 最小int。 | 極端な負数。 | OK |
| [ ] | `TestEvaluateTree_RejectsOutOfRangeScore/score_101` | `EvaluateTree` | 数値上限 | `score=101`。 | range error。 | なし。 | error contains range message | 100超過を拒否すること。 | 最大int。 | 極端な正数。 | OK |
| [ ] | `TestEvaluateTree_RejectsMalformedJSON` | `EvaluateTree` | response parse | Providerが途中で切れたJSONを返す。 | `parse evaluation result` error。 | なし。 | error message | 壊れた構造化出力を成功扱いしないこと。 | valid JSONだがschema不整合。 | 欠落・型違い・unknown field。 | PARTIAL |
| [ ] | `TestEvaluateTree_WrapsLLMError` | `EvaluateTree` | dependency error | Providerが`provider unavailable`を返す。 | `evaluate tree:`でwrapされたerror。 | なし。 | wrapped message | Provider障害を文脈付きで伝播すること。 | `errors.Is`、型付きretryable判定。 | sentinel errorを使う`errors.Is`テスト。 | PARTIAL |
| [ ] | `TestEvaluateTree_NilClientReturnsExplicitFailure` | `EvaluateTree` | nil dependency | `NewEvaluator(nil)`。 | errorなし、0点、不合格、明示summary。 | なし。 | score / passed / summary | LLM未設定をpanicさせず評価不能として返すこと。 | 呼び出し元がこの結果をどう扱うか。 | Worker統合で評価不能のstatus確認。 | PARTIAL |

## 観点別チェックグリッド

| テストケース | 正常系 | 異常系 | 境界値 | 構造化出力 | 矛盾防止 | dependency mock | error wrapping | prompt / schema | 非同期 / cancellation |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestEvaluateTree_DerivesPassedFromScore` | ☑ | ◐ | ☑ | ☑ | ☑ | ☑ | - | ☑ | - |
| `TestEvaluateTree_RejectsOutOfRangeScore` | - | ☑ | ☑ | ☑ | ☑ | ☑ | - | - | - |
| `TestEvaluateTree_RejectsMalformedJSON` | - | ☑ | - | ☑ | - | ☑ | ☑ | - | - |
| `TestEvaluateTree_WrapsLLMError` | - | ☑ | - | - | - | ☑ | ☑ | - | - |
| `TestEvaluateTree_NilClientReturnsExplicitFailure` | - | ☑ | - | - | - | ☑ | - | - | - |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| schema厳格性 | malformed JSONのみ確認。 | field欠落、型違い、unknown fieldを拒否できるschema validation。 |
| cancellation | 未確認。 | canceled context / expired deadlineがProviderへ渡り、元errorを保持すること。 |
| 評価説明 | scoreとpassedのみ重点確認。 | summary/findingsの保持、空値拒否、件数上限。 |
| 入力サイズ | 未確認。 | 巨大treeDataの上限、truncate/chunkingの仕様。 |
| integration | fake LLMのみ。 | Worker経路で評価結果がstatus/logへ反映されるテスト。 |

## 境界値チェックマトリクス

### score

| 対象値 / 条件 | `-1` | `0` | `69` | `70` | `71` | `100` | `101` | missing | wrong type | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| evaluation score | ☑ | ◐ nil-client経由 | ☑ | ☑ | ◐ 95で代表 | ◐ 95で代表 | ☑ | - | - | `TestEvaluateTree_DerivesPassedFromScore`, `TestEvaluateTree_RejectsOutOfRangeScore`, `TestEvaluateTree_NilClientReturnsExplicitFailure` | 0、71、100、missing、string/nullを直接確認。 |

### treeData

| 対象値 / 条件 | empty | valid JSON | malformed JSON text | whitespace | huge | secret-like content | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `treeData` input | - | ☑ | - | - | - | - | `TestEvaluateTree_DerivesPassedFromScore` | empty/whitespace、巨大入力、機密情報をログへ出さないこと。 |
