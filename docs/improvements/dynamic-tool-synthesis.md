# LLM ワーカーによる動的ツール生成 (Dynamic Tool Synthesis)

## 背景

LLM ワーカーは固定されたツール集合 ([orchestrator.go の `[]tool.Tool{...}`](../../apps/worker/pkg/worker/agents/orchestrator.go)) しか持たない。実ドキュメント処理では「この CSV を整形したい」「この特殊な区切りをパースしたい」「base64 を展開したい」といった、その場限りの変換が必要になることがある。現状はこれらを既存ツールの組み合わせで凌ぐか、対応できずに品質が落ちる。

やりたいこと:

- LLM が処理中に「こういう変換が要る」と判断したら、その場で変換ロジック (python / Starlark 関数) を**生成して実行**する
- 有用だった変換は**永続化**して以降のジョブで再利用する (使い捨てで終わらせない)
- 永続化されたものは [llm-eval-runner.md](llm-eval-runner.md) のツールレジストリに乗せ、評価対象にもできる（**依存注意**: これは eval runner のマルチツール対応を前提とするが、現状 eval runner は単一 tool 固定であり、[prompt-variant-eval-contract.md](../contracts/prompt-variant-eval-contract.md) §8 が `knowledge_tree` 以外を明示的にスコープ外と確定している。後述の依存衝突を参照）

設計の主要判断は固まっており、一部は既に実装されている (下記「現在地」)。コード実行環境のセキュリティ判断が最も重く、特に Python 実行を担う executor サービスが未実装である点に注意。

### 現在地 (2026-05-18 時点)

「決定済み」と「実装済み」は別物である。読者は段階1から着手する前提で読まないこと。

| 領域 | 状態 |
| :--- | :--- |
| Starlark analyzer / engine (段階1) | **実装済み**・テスト通過 ([transform package](../../apps/worker/pkg/worker/transform)) |
| `create_transform` メタツール | **配線済み** ([orchestrator.go](../../apps/worker/pkg/worker/agents/orchestrator.go) で Starlark engine に接続) |
| `domain.DynamicTool` (tier / scope / status / version) | **実装済み** ([dynamic_tool.go](../../packages/shared/domain/dynamic_tool.go)、postgres + mock + sqlc) |
| candidate 記録 / 昇格リポジトリ API | **リポジトリ層のみ実装** (`RecordCandidate` / `ListCandidates` / `PromoteCandidate` / `PromoteToGlobal`)。**これを駆動する非同期昇格ワーカー/呼び出し元は未実装** — 昇格は動かない |
| `JobCapability.MaxTransform*` / `UsageLimiter.IncrementTransform*` | **実装済み** |
| executor proto (`executor.pb.go`) | **生成済み**。ただし **executor サービス本体は存在しない** (`apps/` に executor cmd 無し) |
| Python 実行経路 | **未実装**。`create_transform` は Starlark 以外を明示的に拒否する |
| eval runner への動的ツール統合 | **未実装**。eval runner は今も単一 tool 固定 ([runner.go](../../apps/eval/runner/runner.go) は `knowledge_tree` 以外をエラー) |

要約: **動いているのは段階1 (worker 内 Starlark の使い捨て実行) のみ。** 実用変換を担う Python executor、昇格の自動化、eval ゲートはいずれも未稼働。「決定済み事項」節 (後述) は設計判断の確定であって稼働状態ではない。

## 用語

- **エフェメラルツール (ephemeral tool)**: LLM が 1 ジョブ内で生成し、そのジョブ限りで使う変換コード
- **昇格 (promotion)**: エフェメラルツールを永続レジストリに登録し、以降のジョブで再利用可能にすること
- **生成ツール (dynamic tool)**: 昇格されて永続化されたツール

## 全体フロー

```
1. LLM が変換の必要を判断
   └→ meta-tool "create_transform" を呼ぶ (code, language, description, io schema)
2. ランナーが生成コードをサンドボックスで実行 (実行環境は後述・未決定)
   └→ 成否・出力・所要時間・リソース使用量を記録
3. そのジョブ内では即座に再利用可能 (エフェメラル)
4. ジョブ終了時に「昇格候補」として記録
   └→ コード + 入出力サンプル + 使用回数 + risk tier を candidate ストアに保存
5. 昇格判定 (risk tier ベース・後述)
   ├→ tier 1: 自動昇格してレジストリ登録
   ├→ tier 2: 自動昇格するが通知 (通知方法は別途議論)
   └→ tier 3: 人間レビュー必須 (保留状態で待機)
6. 昇格後は llm-eval-runner のツールレジストリに乗り、評価対象になる
```

## メタツール `create_transform`

既存ツールと同じ `functiontool.New` で 1 つ「ツールを作るツール」を追加する。

```go
type CreateTransformArgs struct {
    Name        string `json:"name"`        // 例: "normalize_pipe_delimited"
    Description string `json:"description"` // 何をするか (昇格時のメタデータ)
    Language    string `json:"language"`    // "python" | "starlark"
    Code        string `json:"code"`
    InputSample string `json:"input_sample"`  // 動作確認用
}

type CreateTransformResult struct {
    Output    string `json:"output"`     // input_sample を流した結果
    Reusable  bool   `json:"reusable"`   // 昇格候補に載ったか
    ToolName  string `json:"tool_name"`  // 以降このジョブ内で呼べる名前
}
```

LLM はこれを呼んで変換器を「定義」し、結果を見て採用するか判断する。採用したものは同ジョブ内で `tool_name` として通常ツールのように呼べる。

## 実行環境 (決定: Starlark 組込 + Python executor)

worker は Cloud Run で動く ([llm-eval-runner.md](llm-eval-runner.md) の議論参照)。コンテナはイミュータブルで、LLM 生成コードを本番プロセスで生実行するのは RCE・リソース枯渇・ジョブ間汚染のリスクがある。

### 検討した選択肢 (判断根拠として記録)

| 選択肢 | 隔離度 | 実装コスト | レイテンシ | Python そのまま | 判定 |
| :--- | :--- | :--- | :--- | :--- |
| worker 内で直接 exec | 低 | 最小 | 最小 | ◎ | ✗ 生成コードが worker の SA・DB DSN・GCS 権限を継承。本番不可 |
| WASM 組込 (wazero 等) | 高 | 中 | 小 | △ | ✗ Python をそのまま動かせない (要望と不一致) |
| Starlark 組込 | 高 | 小〜中 | 小 | ✗ | △ 最軽量・安全だが Python 非互換。tier 1 純粋変換のみなら可 |
| **別 Cloud Run サービス (隔離実行器)** | 高 | 中〜大 | 中 (RPC) | ◎ | **採用** |
| gVisor / サンドボックス VM | 最高 | 大 | 中 | ◎ | △ 単独案ではなく採用案の隔離強化オプション |

### 決定

対応言語は **Starlark と Python のみ** とする。shell は実装対象から外す。

- **Starlark**: worker 内の組込ランタイムで実行する。純粋・軽量・決定論的な変換を担当する
- **Python**: 別 Cloud Run サービス「コード実行器 (executor)」で隔離実行する。実用的なテキスト/JSON/CSV 変換を担当する
- **shell**: 非対応。静的解析と再現性のコストが高く、Python で代替できるため扱わない

Python executor を分離する理由:

- Python をそのまま動かせる
- worker の認証情報 (サービスアカウント・DB DSN・GCS 書込) と**完全分離**できる
- 既存 [`cloud_run_service` Terraform モジュール](../../terraform/modules/cloud_run_service/main.tf) を再利用でき、増設パターンが確立済み
- worker → executor の呼び出しは既存の [WorkerDispatcher](../../apps/api/cmd/server/main.go) と同型の RPC をもう一段重ねるだけ

### executor サービスの設計

```
worker (Cloud Run)
  └─ create_transform / 昇格ツール実行
       └─ RPC (connect/HTTP, 入力 + code + language + quota)
            ↓
executor (Cloud Run・新設)
  ├─ サービスアカウント: 最小権限 (DB なし・GCS なし・Secret なし)
  ├─ ingress: internal のみ (worker からのみ到達可)
  ├─ egress: VPC 経由で外向き全遮断 (ネットワークなし)
  ├─ timeout: 短く (例 30s)・CPU/メモリ limit を低く固定
  ├─ イメージ: python3 同梱 (worker とは別 Dockerfile)
  └─ 1 リクエスト = 1 コード実行・状態を持たない
```

必須ガード (どの実装でも担保する):

- **認証情報の非継承**: executor の SA に DB / GCS / Secret Manager の権限を一切付与しない
- **ネットワーク遮断**: egress を VPC コネクタ + ファイアウォールで全拒否 (生成コードが外部送信できない)
- **リソースクォータ**: CPU / メモリ / 実行時間 / 出力サイズに上限。超過は kill して失敗扱い
- **ステートレス**: 実行間でファイル・プロセスを残さない (Cloud Run のインスタンス再利用でも汚染しないよう毎回クリーン)
- **入出力は値渡しのみ**: 入力データと結果文字列を RPC で受け渡す。FS / ボリュームの共有はしない

### エフェメラルと昇格後で同じ runtime を使う

[全体フロー](#全体フロー) のステップ 2 (エフェメラル実行) と昇格後のツール実行は、**同じ language なら同じ runtime** を使う。実行環境が変わると挙動が変わり eval の再現性が崩れるため。

- `language=starlark`: worker 内 Starlark runtime
- `language=python`: executor service

### 結果の受け渡し (worker ⇄ executor)

#### トランスポート: 既存パターンを踏襲

worker は既に Cloud Run 間 RPC を [`HTTPDispatcher`](../../apps/worker/pkg/worker/worker.go) で行っている — connect RPC + `idtoken.NewClient` による IAM 認証 ([worker.go:362](../../apps/worker/pkg/worker/worker.go#L362))。executor もこの**同型**にする。新しい通信方式を作らない。

- connect サービス `ExecutorService` を 1 つ定義 (`buf` 配下に proto 追加)
- worker → executor は idtoken 付き connect クライアント (executor の ingress=internal + IAM invoker で二重に絞る)
- 認証は Cloud Run の IAM。executor 側に DB/Secret が無いので、漏れても二次被害が無い設計と併せる

#### データ受け渡し: 署名付き GCS URL (stdin/stdout 値渡しは不採用)

stdin/stdout のインライン値渡しは筋が悪い:

- connect/gRPC メッセージ上限 (~4MB) に縛られ、ドキュメント変換 (数十 KB〜数 MB・PDF/画像はさらに大) が破綻する
- worker のメモリに入出力を丸ごと載せる (worker は重い処理を持ちたくない)
- バイナリを JSON 文字列に詰めると base64 で 1.33 倍に膨らむ
- 巨大ペイロードを RPC ボディに乗せるとタイムアウト/リトライで全再送

代わりに **worker が署名付き GCS URL を発行して executor に渡す** 方式を採る。**executor に GCS 権限は付与しない** — URL 自体に時限付きの権限が埋まっているため、[必須ガード](#executor-サービスの設計) の「認証情報の非継承」を一切弱めない。

repo には既に前例がある: [`gcsSignedDocumentUploadURLIssuer`](../../packages/shared/app/bootstrap.go#L71) が V4 署名 + IAM 署名を実装済み。これを再利用する (executor 用に汎用化)。

```
ExecuteRequest {
  language        // "python" | "starlark"
  code            // ツール本文 (エフェメラル: 生成直後 / 昇格後: dynamic_tools.code)
  input_url       // 署名付き GCS GET URL (read-only・時限・worker が署名)
  output_url      // 署名付き GCS PUT URL (write-only・時限・worker が署名)
  quota {         // worker が毎回明示的に渡す (executor 側でも上限を二重化)
    timeout_ms
    max_memory_mb
    max_output_bytes
  }
}

ExecuteResponse {
  status        // ok | timeout | oom | nonzero_exit | output_too_large | analyzer_rejected | output_missing
  stderr        // 失敗診断用 (これだけは小さいのでインライン・切り詰め)
  exit_code
  duration_ms
  output_size   // executor が output_url に PUT したバイト数 (0 や未 PUT は output_missing)
  floor_tier    // executor 同梱の静的解析が算出した機械的下限 (tier 判定で使う)
}
```

ポイント:

- **権限の方向を URL で固定**: `input_url` は GET 専用、`output_url` は PUT 専用。executor は input を読む / output へ書く以外できない。バケット内の他オブジェクトには触れない (署名はオブジェクト単位)
- **オブジェクトのスコープ**: 受け渡し用の一時オブジェクトは `gs://<bucket>/transform-io/<workspace_id>/<job_id>/<uuid>` のように workspace/job で名前空間を切る ([名前空間](#名前空間とスコープ-深掘り) の境界と整合)
- **短い TTL**: URL は実行 1 回分の寿命 (例: quota.timeout_ms + マージン)。失効後は誰も読めない
- **ライフサイクル削除**: `transform-io/` プレフィックスに GCS のオブジェクトライフサイクル (例: 1 日で自動削除) を設定。中間データを残さない
- **`stderr` だけは例外的にインライン**: 失敗診断は小さく、GCS 経由にすると失敗時にさらに失敗経路が増えるため RPC で直接返す
- 入力が現実的サイズを超えるほど大きいケースは、その変換を**分割対象**として扱う ([router-job-splitting.md](router-job-splitting.md) と整合)

#### ツール呼び出しから見た流れ

```
LLM が生成ツール foo を呼ぶ
  → registry が (workspace 解決して) dynamic_tools から code/language を引く
  → worker:
      ① LLM が渡した引数を io_schema(input) で検証
      ② 入力を一時 GCS オブジェクトに書く (in)・読み取り専用署名 URL を発行
      ③ 出力先 GCS オブジェクトの書き込み専用署名 URL を発行
      ④ ExecuteRequest を組んで executor を呼ぶ
  → executor: 静的解析(floor算出) → input_url から GET → サンドボックス実行
              → 結果を output_url へ PUT → ExecuteResponse
  → worker: status を検査
       ok        → output オブジェクトを取得し io_schema(output) で検証 → ADK ツール結果として LLM に返す
       失敗系    → エラーを LLM に返す (LLM がリトライ/別手段を選べる)
  → 一時オブジェクトはライフサイクルで自動削除 (worker が即時 delete してもよい)
  → エフェメラル時のみ: 成否・duration・floor_tier を candidate 記録に反映
```

#### io_schema による境界検証

`dynamic_tools.io_schema` (JSON Schema) を**両端で**使う。

- 送信前 (worker): LLM が渡した引数を input schema で検証。壊れた入力を executor に送らない
- 受信後 (worker): `stdout` を output schema で検証。**executor の出力を無検証で LLM に流さない** (生成コードの出力は信頼しない)
- schema 不一致は実行失敗扱い。LLM には「ツールが規約外の出力をした」と返す

これにより executor を信頼境界の外に置いたまま、worker 側で入出力契約を強制できる。

### 段階的導入

executor は本命だがインフラ追加を伴う。段階を踏む:

1. **Starlark 組込で先行検証** — executor 構築前に、worker 内 Starlark で `create_transform` メタツール 〜 tier 1 自動昇格まで通す。インフラ判断を待たず価値検証 (Starlark は決定論的・無権限なので worker 内でも安全)
2. **executor サービス新設** — Terraform で executor を追加。最小権限 SA・ネットワーク遮断を構築
3. **executor へ切替** — `language` 抽象を介して実行先を Starlark から Python executor に拡張

→ Starlark は「executor が来るまでの安全な踏み台」。最終形は executor に一本化する (実行器の二重維持はしない)。

この `language` 抽象を worker のコード構造 (Registry / Runtime) としてどう実装するかは [transform-engine-registry.md](transform-engine-registry.md) で詳述する。

## 永続化と昇格 (方針確定済み)

### 既存 risk tier モデルを流用する

repo には既に risk tier の仕組みがある:

- [util.NormalizeRiskTier](../../packages/shared/util/util.go) — `tier_1` / `tier_2` / `tier_3` の正規化
- [domain.JobExecutionPlan.RequiresApproval](../../packages/shared/domain/types.go) — tier_2 以上で承認要求
- ジョブ承認フロー (`ListJobApprovalRequests` 等) が既に存在

生成ツールにもこの tier を付与し、**昇格を tier で分岐させる**。新しい承認概念を作らず既存フローに寄せる。

### tier ごとの昇格挙動

| tier | 意味の例 | 昇格挙動 | 通知 |
| :--- | :--- | :--- |
| **tier 1** | 純粋関数的な無害変換 (文字列整形・エンコード変換・数値計算) | **自動昇格**しレジストリ登録。人間判断なし | なし or 集計のみ |
| **tier 2** | I/O を伴うが副作用が限定的 (パース・抽出・構造変換) | **自動昇格**するが**通知**する。事後監査前提 | 要 (方法は別途議論) |
| **tier 3** | 外部影響・破壊的・判断が割れるもの | **人間レビュー必須**。保留状態で待機し承認まで非アクティブ | 要 (方法は別途議論) |

### risk tier の判定者・通知方法

判定者は [risk tier の判定 (深掘り)](#risk-tier-の判定-深掘り)、通知方法は [通知方法 (tier 2 / 3)](#通知方法-tier-2--3--未決定) を参照。

## データモデル (ドラフト)

```
dynamic_tools
  tool_id              (ULID)
  name                 (LLM が呼ぶ論理名)
  version              (同一 scope+workspace+name 内で単調増加)
  scope                (workspace | global)
  origin_workspace_id  (生まれた workspace。scope=workspace の解決キー)
  origin_job_id        (どのジョブで生まれたか)
  description
  language             (python | starlark)
  code                 (本文)
  io_schema            (入出力 JSON Schema)
  declared_tier        (LLM 自己申告)
  floor_tier           (executor 静的解析が算出した機械的下限)
  risk_tier            (= max(floor_tier, declared_tier)・実効値)
  status               (candidate | active | held | rejected)
  use_count            (エフェメラル含む使用回数)
  created_at / promoted_at / reviewed_by
```

- レジストリキーは `(scope, origin_workspace_id, name, version)`。LLM 呼び出しは `name` のみ、レジストリが現 job の workspace で解決 ([名前空間](#名前空間とスコープ-深掘り) 参照)
- レジストリ ([llm-eval-runner.md](llm-eval-runner.md) 前提作業 2) は現 workspace の `status=active` + `scope=global` の `status=active` を読み込んで `Build()` に含める。テナント間はデフォルト拒否
- 昇格 = `status` を candidate → active に遷移させる操作。global 昇格は scope を workspace → global にする独立した追加操作
- `risk_tier` は保存時に確定。`declared_tier < floor_tier` だった場合は `floor_tier` 採用 + 監査ログ

## 決定済み事項

> ここに列挙するのは**設計判断の確定**であって稼働状態ではない。実装の進捗は冒頭「現在地」を参照。特に「実行環境」は判断は確定だが executor サービス本体は未実装である。

- **実行環境**: 別 Cloud Run サービス「executor」を新設し隔離実行 ([実行環境セクション](#実行環境-決定-別-cloud-run-サービスによる隔離実行器) 参照)。worker の認証情報と完全分離・ネットワーク遮断・最小権限 SA。Starlark 組込を踏み台に段階導入し、最終的に executor へ一本化
- **永続化と昇格**: 既存 risk tier モデルを流用。tier 1 自動昇格 / tier 2 自動昇格+通知 / tier 3 人間レビュー必須
- **risk tier 判定**: `max(静的解析の floor, LLM 自己申告)`。解析器は executor に同梱、AST ベース、未対応構文は tier_3 フォールバック。判定はセキュリティの単一障害点ではなく実行時隔離が本丸 ([深掘り](#risk-tier-の判定-深掘り) 参照)
- **名前空間**: 既定 workspace スコープで誕生、global は明示の追加昇格のみ。レジストリキー `(scope, origin_workspace_id, name, version)`、再生成は version インクリメント、解決は workspace > global、テナント間デフォルト拒否 ([深掘り](#名前空間とスコープ-深掘り) 参照)
- **結果の受け渡し**: 制御は既存 `HTTPDispatcher` と同型の connect RPC + idtoken 認証。データ本体は **worker が署名した時限 GCS URL** (input=GET専用 / output=PUT専用) で受け渡し、executor に GCS 権限は付与しない (隔離を弱めない)。stdin/stdout 値渡しは不採用。io_schema を worker 側の両端で強制し executor 出力を無検証で LLM に流さない ([受け渡しセクション](#データ受け渡し-署名付き-gcs-url-stdinstdout-値渡しは不採用) 参照)
- **コスト/再帰防止**: `JobCapability` に `MaxTransformCreations` / `MaxTransformRuns` を追加し既存 `UsageLimiter` で強制。メタツールの自己呼び出し禁止・重複生成抑制・失敗予算で封印 ([深掘り](#コストと再帰の暴走防止-深掘り) 参照)
- **構文検証**: 実行前に code を AST パース。失敗は `syntax_error` で即返し、パーサ指摘を添えて LLM を 1 回だけ修正再走 (失敗予算を共有・無限ループ防止)。AST ベース静的解析の前段として一体実装しコスト増ゼロ ([判定パイプライン](#判定パイプライン) 参照)
- **失敗の返し方/eval ゲート**: status 別に LLM への返答を固定 (再試行を煽らない)。修正再生成は機械側で 1 回に制限。昇格前ゲートは tier 別 (tier1=回帰1回 / tier2=+決定論 / tier3=+人間) ([深掘り](#失敗の返し方と-eval-ゲート-深掘り) 参照)
- **昇格発火点/競合**: candidate 記録はジョブ内、tier 判定・昇格はジョブ外の非同期処理。採番は DB 一意制約 `(scope, origin_workspace_id, name, version)` を真実とし FOR UPDATE で二重昇格防止 ([深掘り](#昇格の発火点と競合制御-深掘り) 参照)
- **ランタイム/監査**: executor ランタイムは最小固定 (python ピン留め・許可ライブラリ最小・busybox・ネット系は物理的に非同梱)。`runtime_version` / `input_sample` / 3 種 tier を全保存、kill switch は `status=disabled` 一発 ([深掘り](#ランタイム定義と監査-深掘り) 参照)

## risk tier の判定 (深掘り)

「LLM の自己申告だけ」は過小評価インセンティブ (低 tier の方が自動昇格されて自分の成果が残る) があるため不可。**機械的下限を静的解析で決め、LLM 申告はそれ以上のみ許容**する方式を採る。

### 判定パイプライン

```
生成コード
  │
  ├─(0) パース (構文検証): code を AST にパースする
  │      失敗 → status=syntax_error で即返却。サンドボックス実行に進まない
  │             (GCS URL 発行・サンドボックス起動を空振りさせない)
  │      成功 → AST が得られる = 構文 OK。その AST を (a) に渡す
  │
  ├─(a) 静的解析: (0) の AST を走査して機械的下限 floor を算出
  │      tier_1: 危険シグナル無し
  │      tier_2: I/O・パース系シグナル
  │      tier_3: ネットワーク/サブプロセス/動的eval/FS書込シグナル
  │
  ├─(b) LLM 自己申告: create_transform の declared_tier
  │
  └─ 最終 tier = max(floor, declared_tier)
       declared < floor の場合: 申告を却下しログ (申告者の過小評価を検知)
```

**ステップ (0) パースは独立コストではない。** (a) の静的解析は AST ベース ([後述](#a-静的解析のシグナル-言語別)) なので、どのみち code をパースする。パース成功 = 構文 OK が副産物で得られるため、(0) は (a) の前段として一体で実装する (パース 1 回で構文検証と静的解析の両方を賄う)。

`syntax_error` を実行前に弾く理由:

- 構文エラーのコードをサンドボックス実行ステージまで運ぶのは無駄 (executor サンドボックス起動・worker 側の GCS 署名 URL 発行が空振り)
- LLM へのフィードバックが「実行時に落ちた」より「N 行目で構文エラー」の方が修正再生成の精度が上がる。修正再生成は機械側で 1 回に絞る ([失敗の返し方](#executor-失敗を-llm-にどう返すか)) ため、早く正確なフィードバックほど価値が高い
- パース失敗時は AST が無いので静的解析もできない。tier 算出不能 = そもそも先に進めない

`max()` を採る。LLM が floor より高く申告するのは安全側なので許容。低く申告したら floor に引き上げ + 監査ログ。

なお (0) のパース失敗 (`syntax_error`) は、304 行の「AST 解析が失敗/未対応構文のとき tier_3 フォールバック」とは別物。後者は*解析器が想定外の構文に出会った*ケース (フェイルセキュアで tier_3)。前者は*ユーザーコードが言語として壊れている*ケース (実行不能なので tier 判定以前に弾く)。両者を混同しない。

### (a) 静的解析のシグナル (言語別)

完全な健全性は不要 — **「これがあれば最低 tier_X」という保守的な検出**でよい (誤って高く出る分には安全側)。

Python executor の floor:

| シグナル | python 検出例 | floor |
| :--- | :--- | :--- |
| ネットワーク | `import socket/urllib/http/requests`, `socket(` | tier_3 |
| サブプロセス | `subprocess`, `os.system`, `os.popen`, `__import__` | tier_3 |
| 動的コード実行 | `eval(`, `exec(`, `compile(`, `pickle` | tier_3 |
| FS 書込 | `open(...,'w'/'a')`, `os.remove`, `shutil` | tier_3 |
| FS 読取 | `open(...,'r')`, `os.listdir` | tier_2 |
| パース/構造変換のみ | `json`, `csv`, `re`, 文字列操作のみ | tier_2 |
| 純粋計算のみ | 上記いずれも無し | tier_1 |

Starlark runtime の floor:

| シグナル | starlark 検出例 | floor |
| :--- | :--- | :--- |
| 外部 module 読込 | `load(...)` | analyzer_rejected |
| 危険 builtin / 擬似 IO | `open`, `eval`, `exec`, `os.*`, `socket.*` | tier_3 |
| JSON 構造変換 | `json.decode`, `json.encode` | tier_2 |
| 純粋計算・文字列変換のみ | 上記いずれも無し | tier_1 |

実装は AST ベースを基本とする。Python は標準 `ast` を executor サービス内で実行前に使う。Starlark は worker 内 runtime が parse と AST walk を行う。難読化耐性を完全には保証しないため、後述の実行時サンドボックス (egress 遮断・権限なし) が最終防御線。AST 解析が失敗/未対応構文のときは **tier_3 にフォールバック** (フェイルセキュア)。

### tier の妥当性は実行時隔離で担保される

静的解析を回避されても、executor は egress 全遮断・最小権限 SA・ステートレスなので「ネットワーク送信できない / 認証情報を読めない / 状態を残せない」。tier 判定は**昇格を自動化してよいかの分類**であり、セキュリティの単一障害点ではない (実行隔離が本丸)。

### 判定者の所在

静的解析器は **executor サービスに同梱**する。理由: コード本文を executor に送るのは実行のため不可避なので、解析も同じ信頼境界で行えば worker にコード解析の責務とランタイム依存 (python `ast` 等) を持ち込まずに済む。executor は実行結果と算出 floor tier を worker に返し、worker は `max(floor, declared)` を確定して candidate ストアに記録する。

### 未決定 (tier 判定の残論点)

- Python / Starlark 以外の言語を後で足すかどうか
- 同一コードでも入力次第で危険度が変わるケース。静的解析の限界をどこまで許容するか
- floor を覆して「これは安全」と人間が tier を下げる例外フロー (tier_3 → tier_1 の手動降格) を認めるか

## 通知方法 (tier 2 / 3) — 未決定

既存 CRITICAL 経路 / log-viewer ビュー / ジョブ承認フロー流用のどれか。ユーザーと別途議論予定。

## 名前空間とスコープ (深掘り)

### 既存の workspace 境界モデルに合わせる

repo は既に [`JobCapability`](../../packages/shared/domain/types.go) で「ジョブは特定 workspace の許可された範囲しか触れない」を表現している (`WorkspaceID`・`AllowedDocumentIDs`・`AllowedOperations`・クォータ)。生成ツールも**この境界に揃える** — 別の隔離概念を作らない。

### 決定: 既定は workspace スコープ、昇格で global へ

| スコープ | 意味 | 誰が使えるか |
| :--- | :--- |
| **workspace** (既定) | 生まれた workspace 内でのみ active | その workspace のジョブのみ |
| **global** | 全 workspace で再利用可 | 全ジョブ |

- 生成ツールは**必ず workspace スコープで誕生**する。`origin_workspace_id` を持つ
- global 昇格は「複数 workspace で独立に同等ツールが生成された」「運用者が有用と判断」など**明示の追加昇格**でのみ。自動で global にはしない
- 理由: あるテナントのドキュメント癖から生まれた変換が、無関係なテナントに勝手に効くと品質・情報漏洩の両面でリスク。既定を狭く、広げるのは明示操作

### 名前衝突の解決

レジストリのキーは **`(scope, workspace_id, name, version)`**。

- 同一 workspace 内で同名が再生成されたら **version をインクリメント** (上書きしない)。レジストリは既定で最新 active version を解決
- workspace スコープ同士は `workspace_id` が違えば同名でも衝突しない (自然に分離)
- global と workspace で同名がある場合は **workspace スコープを優先** (テナント固有の上書きを許す。ローカル > グローバルの解決順)
- LLM が呼ぶときは `name` だけ。レジストリが現在の job の `workspace_id` を見て解決する (呼び出し側は scope を意識しない)

### テナント間漏洩の防止

- candidate / active を引くクエリは**必ず `workspace_id` で絞る** (global を除く)。[`JobCapability.AllowsDocument`](../../packages/shared/domain/types.go) が「空なら全許可」なのと違い、ツール解決は**デフォルト拒否** (明示的に own workspace か global のみ)
- executor に渡るのは選択された 1 ツールのコードのみ。レジストリ全体を executor に晒さない
- 悪意ある workspace が他テナントの active ツールを名前指定で呼べないこと (workspace スコープ解決で構造的に不可能) をテストで担保
- global 昇格の経路は tier 判定とは独立した追加ゲート (tier_1 でも自動 global にはしない)

### 未決定 (名前空間の残論点)

- global 昇格の判定者 (運用者手動 / 「N workspace で独立生成されたら候補」の自動検出)
- version 増加の打ち切り (無限に version が増えるのを防ぐ GC)
- workspace 削除時に紐づく生成ツールをどうするか (即削除 / 猶予 / global 化済みは残す)

## コストと再帰の暴走防止 (深掘り)

executor 実行も計算資源を食う。無制限だと LLM が変換を乱造してコスト爆発・ジョブ無限化する。**既存の [`UsageLimiter`](../../apps/worker/pkg/worker/tools/base/usage.go) / [`JobCapability`](../../packages/shared/domain/types.go) に乗せる** — 新しいクォータ機構を作らない。

### JobCapability にカウンタを追加

`JobCapability` は既に `MaxLLMCalls` / `MaxToolRuns` / `MaxItemCreations` を持つ。同型で 2 つ足す:

| フィールド | 意味 | 既定の考え方 |
| :--- | :--- |
| `MaxTransformCreations` | 1 ジョブで `create_transform` を呼べる回数 | 小さく (例 5)。生成は例外的操作という位置づけ |
| `MaxTransformRuns` | 1 ジョブで生成ツール (エフェメラル+昇格済み) を実行できる総回数 | `MaxToolRuns` とは別枠。executor 呼び出し総量の蓋 |

`UsageLimiter` に `IncrementTransformCreations` / `IncrementTransformRuns` を [既存 `increment` パターン](../../apps/worker/pkg/worker/tools/base/usage.go)と同じく追加し、超過で既存と同じ `usage.limit_exceeded` エラーを返してジョブを止める。

### 再帰生成・失敗ループの遮断

- **メタツールはメタツールを呼べない**: 生成コード内から `create_transform` 相当を起動する経路を与えない (executor は connect クライアントを持たない・ネット遮断なので構造的に不可能だが、規約としても明記)
- **同一ジョブ内の重複生成抑制**: 同じ `description` / 近似コードの `create_transform` が連続したら、新規実行せず**直前の結果を返す** (LLM の「失敗→ほぼ同じ物を再生成」ループを断つ)
- **失敗予算**: `create_transform` が連続 N 回失敗したら、そのジョブでは `create_transform` を**封印**し「生成ツールは使えない、既存ツールで進めよ」と LLM に返す (C 節と接続)
- これらの上限は `JobCapability` 由来なので、ジョブ種別 / workspace プランごとに [既存の capability 発行](../../packages/shared/domain/types.go) で調整できる (専用の設定系を作らない)

## 失敗の返し方と eval ゲート (深掘り)

### executor 失敗を LLM にどう返すか

無限リトライさせないことが目的。`ExecuteResponse.status` ([受け渡し](#リクエスト--レスポンス)) ごとに **LLM への返し方を固定**する:

| status | LLM への返し | 意図 |
| :--- | :--- |
| `syntax_error` | パーサのエラー (行番号・メッセージ) を添えて「構文エラー。**この指摘を直して 1 回だけ**再生成せよ」と即座に LLM を再走させる | 構文崩れは確実に直せるので自動修正ループを 1 回回す。実行ステージには進めない |
| `timeout` / `oom` | 「変換が重すぎる。入力を小さく分割するか別手段を使え」 | 同じコードの単純再試行を促さない |
| `nonzero_exit` / `output_missing` | `stderr` 要約を添えて「コードに不具合。**1 回だけ**修正再生成可、それ以上は既存ツールで進め」 | 修正は許すが回数を絞る |
| `analyzer_rejected` | 「このコードは許可されない操作を含む (理由)。安全な手段に切り替えよ」 | 危険コードの再生成を諦めさせる |
| `output_too_large` | 「出力が大きすぎる。分割して返す設計にせよ」 | 暗黙切り詰めではなく設計変更を促す |

#### syntax_error の修正再生成フロー

`syntax_error` は「実行不能だが確実に直せる」唯一のケースなので、専用の自動修正ループを 1 回回す:

```
create_transform(code) 実行
  → executor: パース失敗 → status=syntax_error + stderr(行番号/メッセージ)
  → worker: 失敗予算カウンタを 1 消費し、LLM を再走
       プロンプト: 「直前の code は構文エラー: <stderr>。
                    この指摘だけを修正した完全な code を返せ。仕様は変えるな」
  → LLM が修正版 code を返す
  → executor で再度パース+静的解析
       成功 → 通常フロー (実行・floor 算出) へ
       再び syntax_error / その他失敗 → 修正予算を使い切り → 封印
            (「生成ツールは使えない、既存ツールで進めよ」と LLM に返す)
```

- 修正は **`create_transform` の失敗予算 ([コスト/再帰防止](#再帰生成失敗ループの遮断)) と同じカウンタ**を消費する。syntax_error 専用に無限ループの抜け道を作らない
- 「指摘だけ直せ・仕様は変えるな」と制約し、修正のたびに別物を作る発散を防ぐ
- 1 回直してまた構文エラーなら、それ以上粘らず封印 (LLM が言語を扱えていない兆候)

これらは [プロンプト外出し (llm-eval-runner.md 前提作業 1)](llm-eval-runner.md) のテンプレートに**規約として埋め込む**。失敗予算と合わせ、「修正再生成は 1 回・連続失敗で封印」を機械側でも強制する (プロンプトの善意に頼らない)。

### 昇格前 eval ゲート

> **依存衝突 (未解消)**: 昇格前ゲートは「昇格ツールを eval runner で評価できる」ことを前提にするが、eval runner は現在 `knowledge_tree` 単一 tool 固定で、[prompt-variant-eval-contract.md](../contracts/prompt-variant-eval-contract.md) §8 がマルチツール対応を明示的にスコープ外と確定している。つまり**動的ツールを評価する経路が存在しないまま** active 化される論理的循環がある。これは本ドラフトと prompt-variant-eval-contract が eval 基盤を共有しながら、「eval runner のマルチツール対応を誰が担うか」を**どちらも相手任せにしている**ことに起因する。解消には: (a) eval runner のマルチツール対応を独立タスクとして切り出し所有者を決める、(b) それまで動的ツールの「昇格前 eval ゲート」は eval runner ではなく `create_transform` 内の回帰実行 (input_sample 再実行) のみに限定する、のいずれかを先に確定する必要がある。下表の tier 1 ゲートが (b) と一致するのは偶然ではなく、現状で唯一実行可能な経路だからである。

[llm-eval-runner.md](llm-eval-runner.md) と接続。**tier 別にゲートの強さを変える**:

| tier | 昇格前ゲート |
| :--- | :--- |
| tier 1 | `create_transform` の `input_sample` で **1 回回帰実行が成功**することを必須。これだけ |
| tier 2 | 上記 + io_schema 適合 + 出力が決定論的 (同入力 2 回で同出力) を確認 |
| tier 3 | 上記 + 人間レビュー (既存ジョブ承認フロー流用) まで `held`。eval だけでは active にしない |

回帰に使う `input_sample` は **candidate と一緒に保全必須** (G 節)。これが無いと昇格判定も再現もできない。

## 昇格の発火点と競合制御 (深掘り)

### 発火点: ジョブ終了後の非同期処理

昇格をジョブ実行パス内でやらない (ジョブのレイテンシ・失敗率に昇格ロジックを巻き込まない)。

```
ジョブ実行中:  create_transform 成功時に candidate を status=candidate で記録するだけ
ジョブ終了後:  昇格ワーカー (別経路) が candidate を拾って tier 判定 → 昇格
```

- 記録はジョブ内 (副作用が小さく、ジョブの一部として自然)
- 昇格判定・レジストリ反映は**ジョブ外の非同期処理**。[router-job-splitting.md](router-job-splitting.md) のコールバック方式と同じ思想 (重い後処理をジョブ本体から切り離す)
- tier 1 自動昇格もこの非同期経路。ジョブ成功/失敗に関わらず candidate は評価対象 (失敗ジョブ中に生まれた有用ツールも拾える)

### 競合制御

複数ジョブが同 workspace で同名ツールを同時生成しうる。

- 採番は **DB の一意制約 `(scope, origin_workspace_id, name, version)`** で保証。version はアプリで採るのではなく「現在の最大 version + 1 を INSERT、衝突したら採り直し」で**DB を真実とする**
- 昇格処理は candidate を `SELECT ... FOR UPDATE` 相当でロックし、二重昇格を防ぐ ([既存 postgres リポジトリ](../../packages/shared/repository/postgres/) のトランザクションパターンに合わせる)
- 同名の複数 candidate が同時に昇格対象になったら、**1 つを active・他は version 違いで保留**。最新 active のみ解決されるので機能上は無害

## ランタイム定義と監査 (深掘り)

### executor のランタイムは最小固定

「何が入っているか」を曖昧にしない。曖昧だと静的解析の前提が崩れ、tier 判定が無意味になる。

- **python**: バージョンを固定 (イメージにピン留め)。標準ライブラリ + **明示許可した最小限のみ** (例: なし、もしくは `regex` 程度)。`numpy` 等の重い数値計算ライブラリは初期スコープでは**入れない** (必要が実証されてから許可リストに足す)
- **starlark**: worker binary に組み込まれた `go.starlark.net` の runtime version を固定し、`load` は禁止する
- ネットワーク系ライブラリは「静的解析で弾く」だけでなく**そもそもイメージに入れない** (誘惑と攻撃面を物理的に減らす)
- ランタイム定義 = executor の Dockerfile。**変更はリリース成果物**として扱う ([llm-eval-runner.md](llm-eval-runner.md) の Cloud Run / イミュータブル方針と同じ)。ランタイムが変わると生成ツールの再現性が変わるので、`dynamic_tools` に `runtime_version` を持たせ、昇格時のランタイムを記録する

### 監査・再現性

昇格ツールが後で問題化したとき追跡できること:

- `dynamic_tools` に既出の `origin_job_id` / `origin_workspace_id` / `reviewed_by` に加え、**`input_sample` (生成時の動作確認入力) を保全必須**。これが無いと「なぜ昇格されたか」を再現できず eval も回帰もできない
- `declared_tier` / `floor_tier` / `risk_tier` を**すべて保存** (実効値だけでなく申告と機械判定の差を後から監査できる。過小申告の傾向分析にも使う)
- 緊急無効化 (kill switch): `status` に直接 `disabled` を立てる運用操作を 1 つ用意。レジストリは `active` 以外を `Build()` に含めないので、1 フィールドで即座に全ジョブから外れる
- 監査ログは [logging.md](logging.md) の severity 体系に乗せる (tier 3 昇格・過小申告検知・kill switch 発動は最低 WARN 以上)

## その他の未決定事項

### ライフサイクル

- 使われなくなった active ツールの失効 (use_count 減衰 / TTL)
- eval ([llm-eval-runner.md](llm-eval-runner.md)) で品質が閾値割れしたら自動で `held` に戻すか (kill switch は手動、自動降格は別途)

## 実装前に決めること

主要設計は固まった (実行環境・tier 判定・名前空間・受け渡し・コスト/再帰防止・失敗扱い/eval ゲート・昇格発火点/競合・ランタイム/監査)。残る未決定は ①tier 判定の残論点 (言語拡張・静的解析の限界・手動降格)、②名前空間の残論点 (global 昇格判定者・version GC・workspace 削除時)、③通知方法、④ライフサイクル (自動失効・自動降格)。いずれも段階的最小実装の途中で確定すればよく、着手のブロッカーではない。

段階的な最小実装案 (現在地を反映した進捗付き):

1. ✅ **Starlark runtime**: `create_transform` メタツール + Starlark エフェメラル実行 (昇格なし・使い捨て)。**実装済み**
2. ◐ **candidate ストアへの記録**: `RecordCandidate` 等のリポジトリ API は実装済みだが、`create_transform` 成功時に candidate を記録する**呼び出し配線が未実装**。記録は動いていない
3. ☐ **tier 1 の自動昇格 + レジストリ連携**: `PromoteCandidate` リポジトリ API はあるが、それを駆動する**非同期昇格ワーカーが未実装**。昇格は動かない
4. ☐ **Python executor サービス新設** (Terraform) + `language` 抽象で Starlark / Python を dispatch。proto のみ生成済み、サービス本体なし
5. ☐ tier 2/3 と通知・レビュー導線

### 実装順序の不変条件 (安全論証との整合)

本ドラフトの安全論証 ([tier の妥当性は実行時隔離で担保される](#tier-の妥当性は実行時隔離で担保される)) は **executor の実行時隔離が存在する前提**で「静的解析は単一障害点ではない」と結論している。現状動いている Starlark は worker 内実行だが、Starlark は無権限・決定論的なのでこれは安全 ([段階的導入](#段階的導入) の論拠)。

ここから導かれる不変条件:

- **Python 実行を有効化する前に executor (段階4) を完成させる。** Python を worker 内や隔離不十分な環境で実行してはならない。「Starlark で価値検証 → Python が欲しくなる → executor 未完成のまま Python を足す」という圧力が構造的にかかるが、これを設計として禁じる。段階4 は段階5 の前提であると同時に、**あらゆる Python 経路の前提**である。
- したがって上記リストの番号順は「やってよい順序」であり、4 を飛ばして Python を部分有効化する近道は安全論証を無効化するため認めない。

## 関連

- [llm-eval-runner.md](llm-eval-runner.md) — ツールレジストリ (昇格先) と eval 基盤。生成ツールはレジストリに乗り評価対象になる
- [router-job-splitting.md](router-job-splitting.md) — 同じく未決定事項を抱えるドラフトの書式先例
- [logging.md](logging.md) — tier 2/3 通知の候補経路 (CRITICAL severity)
- [worker-tools-stub.md](worker-tools-stub.md) — 既存ツールの実装状況
