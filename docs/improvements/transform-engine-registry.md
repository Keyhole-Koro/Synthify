# Transform エンジンの言語抽象 (Engine Registry)

## 背景

[dynamic-tool-synthesis.md](dynamic-tool-synthesis.md) は実行環境を **Starlark 組込 + Python executor** に決め、[段階的導入](dynamic-tool-synthesis.md#段階的導入) で「`language` 抽象を介して実行先を Starlark から Python executor に拡張する」と書いた。本文書はその **`language` 抽象を worker のコード構造としてどう実装するか** に絞った続編。インフラ (executor の SA / ネットワーク / RPC) は親文書が持つので繰り返さない。

これも **実装前ドラフト**。Registry の形だけ先に固め、executor 本体は親文書のステージ 2 で着手する。

## いま何が「ハードコード」なのか

現状の組み立ては [orchestrator.go](../../apps/worker/pkg/worker/agents/orchestrator.go) で具体型を直に注入している:

```go
// orchestrator.go (Stage 1)
createTransform, err := process.NewCreateTransformTool(
    b,
    transform.NewStarlarkAnalyzer(),      // ← 単一 analyzer を決め打ち
    transform.NewStarlarkEngine(10*time.Second), // ← 単一 engine を決め打ち
)
```

そして [create_transform.go](../../apps/worker/pkg/worker/tools/process/create_transform.go) は受け取った 1 個の engine と `args.Language` を文字列比較し、不一致なら弾くだけ:

```go
// runCreateTransform (Stage 1)
if transform.Language(args.Language) != engine.Language() {
    res.Error = "language ... is not available yet"
    return res
}
```

`transform.Engine` / `transform.Analyzer` という**インターフェースは既にある**。スケールしないのは抽象の不在ではなく、**「language → (analyzer, engine) の解決が単一実装に固定」**な点。Python を足すたびに orchestrator と runCreateTransform の両方を改修することになる。

> 補足: 旧 `pkg/worker/transform/starlarkrt/` は proto 直結の二重実装で誰からも呼ばれていなかった。本設計の前段として削除済み (transform/ に一本化)。

## 目標

- 新しい language の追加が **Registry への 1 行登録だけ** で済む。orchestrator / runCreateTransform は無改修
- エフェメラル実行と昇格後実行が **同じ language なら必ず同じ runtime** を通る ([親文書の再現性要件](dynamic-tool-synthesis.md#エフェメラルと昇格後で同じ-runtime-を使う))
- Starlark (in-process) と Python (executor RPC) が **同一インターフェースの裏** に隠れ、呼び出し側から実行先が見えない
- 段階移行で **既存の Starlark 経路の挙動を一切変えない** (Registry 導入はリファクタであって機能変更ではない)

## 設計: Runtime Registry

### 中心の抽象

`Engine` と `Analyzer` を language 単位で束ねた `Runtime` を導入し、Registry が language → Runtime を解決する。

```go
// package transform

// Runtime は 1 言語ぶんの解析+実行をまとめる。in-process (Starlark) でも
// RPC 越し (Python executor) でも、呼び出し側はこの裏側を区別しない。
type Runtime interface {
    Language() Language
    Analyze(code string) AnalysisResult     // 既存 Analyzer と同契約
    Execute(req ExecRequest) ExecResult     // 既存 Engine と同契約
}

// Registry は language → Runtime を解決する。登録は起動時のみ (実行時不変)。
type Registry struct {
    runtimes map[Language]Runtime
}

func (r *Registry) Register(rt Runtime)            // 起動時に呼ぶ
func (r *Registry) Resolve(lang Language) (Runtime, bool)
func (r *Registry) Languages() []Language          // LLM へ提示する許可言語
```

`Runtime` は新しい契約を作らず、**既存の `Analyzer.Analyze` と `Engine.Execute` をそのまま 1 インターフェースに合成しただけ**。Starlark 側は既存の `StarlarkAnalyzer` / `StarlarkEngine` を薄くラップすれば済み、挙動は不変。

### 呼び出し側の変化

orchestrator は Registry を 1 個組んで渡すだけになる:

```go
// orchestrator.go (Registry 導入後)
reg := transform.NewRegistry()
reg.Register(transform.NewStarlarkRuntime(10 * time.Second))
// Stage 2 でここに 1 行足すだけ:
// reg.Register(transform.NewPythonExecutorRuntime(executorClient))

createTransform, err := process.NewCreateTransformTool(b, reg)
```

runCreateTransform は文字列比較を Registry 解決に置き換える:

```go
func runCreateTransform(ctx, reg *transform.Registry, args CreateTransformArgs) CreateTransformResult {
    rt, ok := reg.Resolve(transform.Language(args.Language))
    if !ok {
        res.Status = string(transform.StatusNonzeroExit)
        res.Error = fmt.Sprintf("language %q is not available; supported: %v",
            args.Language, reg.Languages())
        return res
    }
    an := rt.Analyze(args.Code)   // 以降のロジックは現状と同一
    ...
    out := rt.Execute(transform.ExecRequest{Code: args.Code, Input: args.InputSample})
    ...
}
```

これ以降 **language を増やしても runCreateTransform は触らない**。許可言語一覧 (`reg.Languages()`) は create_transform のツール説明や失敗メッセージにそのまま使え、LLM へ提示する対応言語が登録と自動で同期する。

### 昇格後の実行も同じ Registry を通す

[親文書のステージ](dynamic-tool-synthesis.md#段階的導入) で昇格ツールを実行するパスも、**同じ Registry を共有**する。エフェメラル実行 (`create_transform`) と昇格後実行で別々に runtime を選ぶ実装にすると、親文書の再現性要件 (同 language = 同 runtime) がコード上保証されない。Registry を単一の解決点にすることでこの不変条件を構造的に強制する。

```
create_transform (エフェメラル) ─┐
                                  ├→ Registry.Resolve(lang) → Runtime
昇格ツールの実行 (再利用) ────────┘     (同 lang なら必ず同一 Runtime)
```

## 段階移行 (親文書のステージに対応)

親文書の[段階的導入](dynamic-tool-synthesis.md#段階的導入) 1〜3 にこの Registry をどう挟むか:

| 親文書ステージ | 本設計でやること | 挙動変化 |
| :--- | :--- | :--- |
| 1. Starlark 先行検証 (現状) | `Runtime` / `Registry` を追加。`StarlarkRuntime` で既存 engine/analyzer をラップ。orchestrator と runCreateTransform を Registry 経由に書換 | **なし** (純リファクタ。Starlark の解析/実行は同一コードを通る) |
| 2. executor サービス新設 | `PythonExecutorRuntime` を実装 (親文書の RPC + 署名付き GCS URL を裏に隠す)。**まだ登録しない** | なし |
| 3. executor へ切替 | orchestrator で `reg.Register(NewPythonExecutorRuntime(...))` を 1 行追加 | Python が解禁。Starlark は不変 |

ステージ 1 は機能を足さない。Registry を入れても Starlark 経路のテストが全て同じ結果になることが受け入れ条件。新言語の価値はステージ 3 まで出ないが、ステージ 1 で構造を入れておくことで 2→3 が「1 行追加」になる。

## 設計上の判断

- **`Runtime` は既存 2 インターフェースの合成に留め、新契約を作らない** — `ExecRequest`/`ExecResult`/`AnalysisResult` ([transform.go](../../apps/worker/pkg/worker/transform/transform.go)) は proto 非依存の安定型として既に設計されている ([transform.go の意図的な proto 分離コメント](../../apps/worker/pkg/worker/transform/transform.go))。Registry はこの型をそのまま運ぶ箱で、新しいデータ型を増やさない。
- **Registry は起動時不変** — 実行時の動的登録 (LLM が language を増やす等) はしない。登録集合 = デプロイで固定。これにより `reg.Languages()` を信頼境界として使え、未知 language は Resolve 失敗で安全側に倒れる。
- **executor の隔離詳細は Runtime 実装の内側に閉じる** — 署名付き GCS URL・idtoken RPC・クォータ二重化 ([親文書](dynamic-tool-synthesis.md#結果の受け渡し-worker--executor)) は `PythonExecutorRuntime` の実装詳細。呼び出し側 (runCreateTransform) は in-process か RPC かを知らない。
- **shell は登録しない** — 親文書の決定通り対象外。Registry に無い = 自動的に拒否される (特別扱い不要)。

## やらないこと

- executor サービス本体の実装 — 親文書ステージ 2。本文書は worker 内の差込口だけ
- 実行時の動的 language 登録 — 信頼境界を壊すため対象外
- Starlark の機能拡張 (出力サイズ上限・ステップ上限等) — 削除した starlarkrt にあった機能だが、必要になった時点で `StarlarkRuntime` の責務として別途検討。本設計のスコープ外
- 永続化・昇格・risk tier — [親文書](dynamic-tool-synthesis.md#永続化と昇格-方針確定済み)が持つ
