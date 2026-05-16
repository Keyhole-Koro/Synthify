# LLM 出力評価・プロンプト実験基盤 (Eval Runner)

## 背景

LLM ワーカーの各ツール (`goal_driven_synthesis`, `summary`, `briefing`, `critique`, `merging` 等) はプロンプトをコード内にハードコードしている。例: [synthesis.go:61](../../apps/worker/pkg/worker/tools/process/synthesis.go#L61) の `SystemPrompt`。

このため以下ができない。

- プロンプトを変更したときに出力品質が良くなったか悪くなったかを定量的に比較できない
- プロンプトの変更がリグレッションを起こしていないか確認できない (本番ジョブを流すまで分からない)
- 複数のプロンプト案 (バリアント) を同じ入力に対して横並びで試せない
- モデル変更 (Gemini のバージョン更新) の影響を測れない

`LOG_LLM_PAYLOAD=true` ([config.go](../../packages/shared/config/config.go) の `LLM.LogPayload`) でプロンプトと応答をログに吐けるようになったが、これは手動デバッグ用であり、体系的な評価の仕組みではない。

## 目的

プロンプト・モデルを変えながら、固定のテストケースに対して LLM ツールの出力品質を**定量評価**し、バリアントを**横並び比較**できる CLI 基盤を作る。

## 設計方針

### 配置: `apps/eval` として独立 CLI

```
apps/eval/
  cmd/main.go          # CLI エントリポイント
  cases/               # テストケース定義 (YAML)
  testdata/            # 入力 fixture (chunks.json など)
  runner/              # ケース実行 + スコアリング
  judge/               # LLM-as-Judge 評価器
  report/              # 結果出力 (table / JSON / diff)
```

- worker 本体には組み込まず独立 CLI にする。CI でも手元でも回せるようにするため
- 既存のツール実装 (`tools/process/*`) を直接呼び出すのではなく、後述の「ハードコード排除」を前提に `llm.Client` を直接叩く

「ハードコードしない」には 2 軸ある。両方とも eval runner の前提作業となる。

1. **プロンプト** — 各ツールの `SystemPrompt` と orchestrator の `Instruction` がコードにリテラルで埋まっている
2. **ツールの配線** — [orchestrator.go:60-159](../../apps/worker/pkg/worker/agents/orchestrator.go#L60-L159) で全ツールを `New*Tool()` で生成し `[]tool.Tool{...}` に手で並べている

### 前提作業 1: プロンプトの外出し (Cloud Run 前提で `go:embed`)

#### Cloud Run の制約が形式を決める

worker は Cloud Run で動く想定 (イミュータブルなコンテナ / ステートレス / スケールゼロ)。プロンプトの保持方法はこの制約から逆算する。

| 形式 | Cloud Run 適性 |
| :--- | :--- |
| **`go:embed` + テンプレート** | ◎ イメージに焼き込まれる。インスタンスが N 個に増えても全て同一プロンプト。コールドスタートで追加 I/O ゼロ。**イメージタグ = プロンプトバージョン**で再現性が完全。ロールバックは `git revert` + 再デプロイで完結 (DB 状態に依存しない)。 |
| **GCS/設定ファイルを起動時ロード** | △ コールドスタートのたびに外部 I/O。GCS 障害が worker 起動障害に直結。インスタンスごとにバージョンがズレうる。 |
| **DB / リモート管理画面** | △ worker コールドスタートが DB 依存になる。プロンプト変更が無監査で本番に効き、ロールバックが Git 履歴と乖離する。 |

→ **`go:embed` を採用。** プロンプトはコードと同じリリース成果物として扱う。動的変更が後で必要になったら「embed をデフォルト + リビジョン単位の環境変数でオーバーライド」の二段にできる (Cloud Run の env var もリビジョン管理に乗るため再現性を保てる)。

#### 構造

```go
// apps/worker/pkg/worker/prompts/prompts.go
package prompts

import "embed"

//go:embed templates/*.tmpl
var fs embed.FS

type Template struct {
    Name   string
    System string // templates/<name>.system.tmpl
    User   string // templates/<name>.user.tmpl
}

func (t Template) Render(vars any) (system, user string, err error) { ... }
```

```
prompts/
  prompts.go
  templates/
    synthesis.system.tmpl
    synthesis.user.tmpl
    summary.system.tmpl
    ...
```

ツール側は `prompts.Get("synthesis").Render(vars)` を呼ぶだけにする。これにより:

- eval runner が同じ `prompts` パッケージを import して**本番と同一**プロンプトを再現できる
- バリアント比較は eval 側の `templates-variant/` を embed して差し替えるだけで済む (本番イメージには未採用バリアントが入らない)

### 前提作業 2: ツールレジストリ

現状 orchestrator が全ツールを手で生成・列挙している。レジストリに登録し、orchestrator は名前のリストから組み立てる形にする。

```go
// apps/worker/pkg/worker/tools/registry.go
type Factory func(*base.Context) (tool.Tool, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) { registry[name] = f }

func Build(b *base.Context, names []string) ([]tool.Tool, error) { ... }
```

各ツールパッケージは `init()` で `tools.Register("goal_driven_synthesis", NewSynthesisTool)` する。orchestrator は配線リストを定数 (将来は宣言的定義) から受け取り `tools.Build(b, names)` で組み立てる。

これで **ツール追加** が 2 つの粒度で扱えるようになる。

| 粒度 | やり方 | 用途 |
| :--- | :--- | :--- |
| **開発時の新ツール追加** | ツールパッケージで `tools.Register(...)` 一行 + import | orchestrator を触らず機能拡張 |
| **実行時のツール集合制御** | `tools.Build(b, names)` の `names` をジョブ設定 / リクエストから渡す | ジョブ単位で「このツールは使わせない」、A/B で「ツール X 有無」を比較 |

実行時制御は新規ロジック不要 (レジストリ + 名前リストで自然に実現する)。これにより:

- 新ツール追加が「`Register` 一行 + import」で済み、orchestrator の編集が不要になる
- eval runner が**一部のツールだけ差し替えた**サブセットで agent を組める (例: synthesis だけバリアント版に置換)
- ツール集合自体を評価軸にできる (ツールを 1 つ外したら品質がどう変わるか)

### ADK を使い続けるか (検討結果: 継続)

eval を設計するにあたり「そもそも ADK ([google.golang.org/adk](https://pkg.go.dev/google.golang.org/adk)) を使うべきか」を検討した。**結論は継続。**

#### ADK が現状担っている責務

| 機能 | 使用箇所 | 自前化のコスト |
| :--- | :--- | :--- |
| エージェント自律ループ (LLM→ツール選択→実行→再 LLM) | [orchestrator.go:296](../../apps/worker/pkg/worker/agents/orchestrator.go#L296) `runner.Run` | 高: tool-calling のループ・終了判定・履歴管理を再実装 |
| `Before/AfterModel` `Before/AfterTool` コールバック | [orchestrator.go:160-219](../../apps/worker/pkg/worker/agents/orchestrator.go#L160-L219) | 高: usage 制限・working memory 注入・**checkpoint 復帰**の差し込み点がこれに依存 |
| Go 関数 → ツールスキーマ自動生成 | 全 `functiontool.New` 呼び出し (15+ ツール) | 中: 各ツールの JSON Schema を手書き or 別ライブラリ |
| セッション/会話履歴 | [worker.go:80](../../apps/worker/pkg/worker/worker.go#L80) `session.InMemoryService` | 低〜中 |

#### 継続の理由

- **置換コストが価値を上回る**: 自律ループとコールバック機構を自前化すると、checkpoint 復帰 ([job-checkpoint-spec](../architecture/job-checkpoint-spec.md)) と usage 制限の差し込み点をすべて作り直すことになる。これは eval の目的 (出力品質の評価) とは無関係なリスク
- **eval は ADK の有無に直交させられる**: eval runner は「ツール単体評価」と「agent 全体評価」の 2 レイヤーで構成する (下記)。前者は ADK を経由しないので、ADK を残したまま決定論的な評価ができる
- **乗り換え判断は eval ができてから**: eval 基盤があれば「ADK あり / 自前ループ」を**同じ指標で比較**できる。今ここで脱却を決めるより、まず計測基盤を作る方が順序として正しい

#### eval への影響: 2 レイヤー構成にする

ユーザー方針により agent 全体も評価対象に含める。eval runner は次の 2 レイヤーを持つ。

| レイヤー | 対象 | ADK | 特性 |
| :--- | :--- | :--- | :--- |
| **Tool eval** | 個別ツール (synthesis 等) を `llm.Client` 直叩き | 経由しない | 決定論寄り・安価・プロンプト差し替えの主戦場 |
| **Agent eval** | orchestrator を ADK runner ごと走らせ最終ツリーを評価 | 経由する | エンドツーエンド・高コスト・ツール順序や自律判断の質を見る |

Tool eval を主軸 (CI nightly + バリアント比較)、Agent eval は重いので手動トリガー + 代表ケースのみに絞る。

### ケース定義 (YAML)

```yaml
name: synthesis_api_spec
tool: synthesis
input:
  document_id: doc_001
  instruction: "技術仕様書として整理して"
  chunks: ./testdata/api_spec_chunks.json
expect:
  schema_valid: true            # 決定論的: JSON が Schema に適合するか
  min_items: 3                  # ルールベース: item 数の下限
  max_depth: 4                  # ルールベース: 階層の深さ上限
  must_contain_titles:          # ルールベース: 特定トピックを含むか
    - "認証"
    - "エラーハンドリング"
  judge_min_score: 0.7          # LLM-as-Judge: 0.0-1.0 のしきい値
```

### 評価レイヤー (3 段階)

| レイヤー | 内容 | 特性 |
| :--- | :--- | :--- |
| **Schema 検証** | 出力 JSON が期待スキーマに適合するか | 決定論的・高速・無料 |
| **ルールベース** | item 数 / 階層 depth / 必須タイトル含有など | 安定・ドメイン知識が必要 |
| **LLM-as-Judge** | 別の LLM 呼び出しで品質を 0.0-1.0 採点 | 品質を測れる・有料・分散あり |

Schema + ルールベースを必須レイヤーとし、LLM-as-Judge は `judge_min_score` が定義されたケースのみ実行する (コスト制御)。

### バリアント比較

バリアントは eval 側に embed したテンプレート群 (`apps/eval/templates-variant/`) で持つ。本番イメージ ([前提作業 1](#前提作業-1-プロンプトの外出し-cloud-run-前提で-goembed)) には混入しない。

```bash
# プロンプト A/B を同一入力で比較
go run ./apps/eval/cmd \
  --case=synthesis_api_spec \
  --variant=synthesis_v1 \
  --variant=synthesis_v2
```

出力イメージ:

```
CASE: synthesis_api_spec
                       v1      v2
schema_valid           PASS    PASS
items                  5       7
max_depth              3       4
judge_score            0.72    0.81   <- v2 wins
duration               2.1s    2.4s
tokens (in/out)        1.2k/3k 1.2k/3.4k
```

### CI 連携

- `eval` ディレクトリのケースを CI で nightly 実行
- `judge_min_score` 未満 / `schema_valid` 失敗で fail
- LLM 呼び出しが必要なので PR ごとではなく nightly + 手動トリガー

## 未決定事項

### 1. LLM-as-Judge のモデルと評価プロンプト

- 評価に使うモデルは本番と同じ Gemini か、別系統 (バイアス回避) か
- 評価プロンプトの設計 (rubric をどう与えるか) 自体が品質を左右する
- スコアの分散をどう扱うか (複数回実行して平均? n=1 で許容?)

### 2. テストケースの入力 fixture の用意

- 実ドキュメントから chunks を抽出して `testdata/` に固定する必要がある
- 個人情報・著作物を含まないサンプルをどう用意するか
- chunk 抽出自体もパイプラインの一部なので、どこを固定点にするか (生 PDF? 抽出済み chunks?)

### 3. 外出し・レジストリ化のスコープと順序

- 全ツール (synthesis / summary / briefing / critique / merging) を一度に外出しするか段階的か
- [worker-tools-stub.md](worker-tools-stub.md) でツール自体がまだ簡易実装のものがある。外出しの順序を stub 解消と揃えるか
- 前提作業 1 (プロンプト) と 2 (レジストリ) のどちらを先にやるか。eval の最小価値はプロンプト差し替えなので 1 を先行で問題ないか

### 4. ゴールデン出力の保持

- ルールベースだけでなく「前回の出力と diff」を取りたいケースがある
- ゴールデンファイルを repo に commit するか (LLM は非決定的なので diff が常に出る懸念)

## 決定済み事項

- **プロンプト保持形式**: `go:embed` + テンプレート。Cloud Run のイミュータブルコンテナ前提で、プロンプトをリリース成果物として扱う ([前提作業 1](#前提作業-1-プロンプトの外出し-cloud-run-前提で-goembed) 参照)
- **「ハードコードしない」のスコープ**: プロンプト外出し + ツールレジストリの 2 軸。宣言的パイプライン化 (ツール列・順序まで YAML 化) は今回のスコープ外とする
- **ツール追加**: レジストリで「開発時の新ツール追加 (`Register` 一行)」と「実行時のツール集合制御 (`Build` の名前リスト)」の両粒度を満たす。後者に専用ロジックは作らない
- **ADK**: 継続。自前ループ化は checkpoint 復帰・usage 制限の差し込み点を作り直すコストが eval の目的に見合わない。乗り換え判断は eval 基盤ができて同一指標で比較できるようになってから
- **eval の構成**: Tool eval (ADK 非経由・主軸) と Agent eval (ADK 経由・手動トリガー) の 2 レイヤー

## 実装前に決めること

未決定事項のうち、最低でも **2 (fixture)** と **3 (スコープ・順序)** を固める必要がある。前提作業 1 (プロンプトの `go:embed` 外出し) が eval の最小前提なので、そこから着手する。

## 関連

- [tool-calling-tests.md](tool-calling-tests.md) — エージェントが正しいツールを呼ぶかのテスト。こちらは「ツールの出力品質」が対象で、関心が異なるが fixture を共有できる可能性がある
- [worker-tools-stub.md](worker-tools-stub.md) — 評価対象ツールの実装状況
- [logging.md](logging.md) — `LOG_LLM_PAYLOAD` によるペイロードダンプ (手動デバッグ用)
