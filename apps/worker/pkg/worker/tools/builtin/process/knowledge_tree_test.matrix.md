# テストマトリクス: `knowledge_tree_test.go`

このマトリクスは、`knowledge_tree_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

> **読み方の注意**: 下の「テストケース表」は既存テストが確認している内容を写したものです。
> テストがない関数・分岐は表に現れないため、「インターフェース網羅チェック」「依存エラー軸」「未テスト分岐 (GAP)」も併読してください。
> カバレッジ数値は未計測 (2026-07-18)。実測時は `go test -coverprofile` の値へ更新すること。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

## インターフェース網羅チェック (`knowledge_tree.go`)

| 関数 / 経路 | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `NewGenerateKnowledgeTreeTool` | ✅ 1件 | 未計測 | PARTIAL | repositoryから既存treeを取得する成功/失敗、入力JSON不正、marshal error。 |
| `GenerateKnowledgeTree` | ✅ 5件 | 未計測 | OK | production prompt loader失敗。 |
| `GenerateKnowledgeTreeWithRenderer` | ◐ `GenerateKnowledgeTree`経由 | 未計測 | PARTIAL | nil renderer、renderer.Render error、明示variant renderer。 |
| `generateKnowledgeTree` | ❌ なし | 未計測 | GAP | 薄いwrapperだが直接の回帰確認なし。 |
| `deterministicKnowledgeTree` | ◐ tool fallback経由 | 未計測 | PARTIAL | 複数chunk、見出しあり、360 rune truncate、非連番chunk index。 |
| object形式 `{items:[...]}` parse | ✅ 1件 | 未計測 | OK | extra fields、null items。 |
| top-level array parse | ✅ 1件 | 未計測 | OK | empty arrayは別テストでobject形式のみ。 |
| malformed / empty LLM output | ✅ 2件 | 未計測 | OK | tool wrapper経由でのfallbackはprovider errorだけ。 |
| usage propagation | ✅ 3件 | 未計測 | OK | fallback後の課金・telemetry分類。 |

## 依存エラー軸 (dependency returns error)

外部依存は `llm.Client`、`prompts.Renderer`、`base.Context.Repo`。☑=テスト有 / ◐=間接 / ❌=未テスト。

| 関数 | LLM error | malformed LLM response | prompt load/render error | repository error | context canceled / deadline |
| --- | --- | --- | --- | --- | --- |
| `GenerateKnowledgeTree` | ☑ `TestGenerateKnowledgeTree_ReturnsProviderErrorAndUsage` | ☑ malformed / empty | ❌ | - | ❌ |
| `GenerateKnowledgeTreeWithRenderer` | ◐ default renderer経由 | ◐ | ❌ | - | ❌ |
| `NewGenerateKnowledgeTreeTool.Run` | ☑ fallback確認 | ◐ provider errorのみ | ❌ | ❌ | ❌ |

→ LLM errorの直接伝播とtool fallbackは確認済み。renderer/repository/cancellation軸は未確認。

## 未テスト分岐 (GAP) — テストケース表に現れない経路

| 場所 | 分岐 | 期待挙動 | なぜ重要か |
| --- | --- | --- | --- |
| `NewGenerateKnowledgeTreeTool.Run` | 入力JSONのunmarshal失敗 | errorを返し、LLMを呼ばない。 | Agentが不正tool argsを生成した際の安全な失敗。 |
| `NewGenerateKnowledgeTreeTool.Run` | `Repo.GetTreeByWorkspace`成功 | 既存nodeを`ExistingNodes`としてpromptへ渡す。 | 重複nodeを作らずmerge判断する主要機能。 |
| `NewGenerateKnowledgeTreeTool.Run` | `Repo.GetTreeByWorkspace`失敗 | best effortで既存nodeなしとして生成を継続する。 | repository一時障害でjob全体を落とさない現行仕様。 |
| `GenerateKnowledgeTreeWithRenderer` | nil LLM | `llm not configured` error。 | provider初期化漏れの明示検知。 |
| `GenerateKnowledgeTreeWithRenderer` | nil renderer | `prompt renderer not configured` error。 | variant/eval wiring不備の明示検知。 |
| `GenerateKnowledgeTreeWithRenderer` | renderer.Render error | errorをそのまま返し、LLMを呼ばない。 | prompt template破損をProvider障害と混同しない。 |
| `NewGenerateKnowledgeTreeTool.Run` | malformed JSON / empty items | 決定論的fallbackへ移行する。 | ProviderがHTTP成功で壊れた出力を返すケース。 |
| `deterministicKnowledgeTree` | 360 rune超過 | descriptionをtruncateしHTMLも同値から生成する。 | fallbackで過大contentが保存されるのを防ぐ。 |
| `deterministicKnowledgeTree` | 複数chunk /非連番index | local/source IDが衝突せず入力indexに対応する。 | chunking結果が必ず0始まり連番とは限らない。 |

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestGenerateKnowledgeTree_ObjectResponseAndUsage` | `GenerateKnowledgeTree` | 正常系 / structured output / usage | Fake LLMが`{"items":[...]}`とtoken usageを返す。 | itemを1件返しusageを保持する。 | なし。 | item title/localID、usage一致、prompt非空、schema指定 | production renderer、object response、usage伝播。 | prompt全文、existing nodes、schema実validation。 | promptにchunk/instruction/existing nodeが含まれること。 | PARTIAL |
| [ ] | `TestGenerateKnowledgeTree_AcceptsTopLevelArray` | `GenerateKnowledgeTree` | compatibility | Fake LLMがtop-level item arrayを返す。 | arrayをitemsとして受理する。 | なし。 | item count/title | 旧/異種Providerのarray出力互換。 | empty array、複数item、nested parent。 | top-level empty arrayと複数階層。 | OK |
| [ ] | `TestGenerateKnowledgeTree_RejectsMalformedJSON` | `GenerateKnowledgeTree` | parse error | 途中で切れたJSON。 | errorを返す。 | なし。 | `err != nil` | 壊れた構造化出力を成功扱いしない。 | error種別、tool wrapperのfallback。 | error messageとfallback経路を確認。 | PARTIAL |
| [ ] | `TestGenerateKnowledgeTree_RejectsEmptyItems` | `GenerateKnowledgeTree` | empty output | `{"items":[]}`。 | `llm returned no items` error。 | なし。 | error message | schema-validでも意味的に空の出力を拒否。 | top-level empty array、null。 | `[]`, `{"items":null}`比較。 | PARTIAL |
| [ ] | `TestGenerateKnowledgeTreeTool_FallsBackOnLLMError` | `NewGenerateKnowledgeTreeTool.Run`, `deterministicKnowledgeTree` | fallback / security / usage | LLM error、空heading、script文字列を含む1chunk。 | toolは成功し、`Section 1` itemを返す。HTMLはescapeされusageを保持。 | fallback item生成。 | title、escaped content、source chunk ID、usage | Provider障害時の継続、XSS文字列escape、fallback ID。 | fallback発生のtelemetry、複数chunk、truncate。 | fallback reasonを結果/ログへ記録するテスト。 | PARTIAL |
| [ ] | `TestGenerateKnowledgeTree_ReturnsProviderErrorAndUsage` | `GenerateKnowledgeTree` | dependency error / usage | Fake LLMがsentinel errorと部分usageを返す。 | 元errorを保持しusageを返す。 | なし。 | `errors.Is`, usage一致 | direct functionがProvider errorを握りつぶさないこと。 | retry、rate limit分類、cancellation。 | typed provider errorとcontext error。 | PARTIAL |

## 観点別チェックグリッド

| テストケース | 正常系 | 異常系 | structured output | compatibility | prompt / schema | usage | fallback | HTML安全性 | repository | cancellation |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestGenerateKnowledgeTree_ObjectResponseAndUsage` | ☑ | - | ☑ | - | ☑ | ☑ | - | - | - | - |
| `TestGenerateKnowledgeTree_AcceptsTopLevelArray` | ☑ | - | ☑ | ☑ | ◐ | - | - | - | - | - |
| `TestGenerateKnowledgeTree_RejectsMalformedJSON` | - | ☑ | ☑ | - | - | - | - | - | - | - |
| `TestGenerateKnowledgeTree_RejectsEmptyItems` | - | ☑ | ☑ | - | - | - | - | - | - | - |
| `TestGenerateKnowledgeTreeTool_FallsBackOnLLMError` | ◐ | ☑ | - | - | - | ☑ | ☑ | ☑ | - | - |
| `TestGenerateKnowledgeTree_ReturnsProviderErrorAndUsage` | - | ☑ | - | - | - | ☑ | - | - | - | - |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| existing tree merge context | 未確認。 | Repoから既存nodeを取得し、LLM request promptにID/title/descriptionが入ること。 |
| prompt renderer | default rendererの正常系のみ。 | nil renderer、template error、variant renderer。 |
| tool input validation | valid JSONのみ。 | malformed JSON、必須ID欠落、空chunks。 |
| fallback coverage | Provider error1種類のみ。 | malformed JSON、empty items、context error時にfallbackすべきかの仕様化。 |
| fallback observability | usageのみ保持。 | fallback flag/reasonがlogまたはresultに残ること。 |
| cancellation / timeout | 未確認。 | canceled contextとdeadline exceededをfallbackせず返すかを決めてテスト。 |
| tree invariants | item存在のみ。 | local ID一意、parent存在、循環なし、source chunk存在、最大深さ。 |
| security | fallback HTML escapeのみ。 | LLM生成HTMLのsanitize境界、危険属性、override CSS。 |

## 境界値チェックマトリクス

### item count / chunk count

| 対象値 / 条件 | `0` | `1` | typical multiple | large | malformed/null | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| LLM output items | ☑ object形式 | ☑ | - | - | ◐ malformed JSON | `TestGenerateKnowledgeTree_ObjectResponseAndUsage`, `TestGenerateKnowledgeTree_AcceptsTopLevelArray`, `TestGenerateKnowledgeTree_RejectsEmptyItems` | top-level 0、複数item、大量item、null。 |
| input chunks | - | ☑ | - | - | - | 全正常テスト | 0 chunks、複数chunk、大量chunk。 |

### scoreではなく文字列 / ID / content

| 対象値 / 条件 | empty | whitespace | typical | HTML/script | long > 360 runes | malformed | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| chunk heading | ☑ fallbackで空 | - | ☑ | - | - | - | object/array/fallback tests | whitespace-only heading、長大heading。 |
| chunk text | - | - | ☑ | ☑ fallback escape | - | - | `TestGenerateKnowledgeTreeTool_FallsBackOnLLMError` | empty text、360境界、Unicode truncate。 |
| document ID | - | - | ☑ | - | - | - | 全テスト | empty/malformed IDとsource ID生成。 |
| LLM JSON | - | - | ☑ | - | - | ☑ | object/array/malformed tests | null、wrong field type、unknown fields。 |

### usage

| 対象値 / 条件 | `0` | typical | partial usage on error | negative | huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| input/output tokens | ◐ fallback output=0 | ☑ | ☑ | - | - | object/usage、fallback、provider error tests | both zero、negative防止、overflow、Provider別usage semantics。 |
