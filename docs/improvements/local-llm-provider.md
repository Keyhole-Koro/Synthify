# Local LLM Provider — サブスク枠を使う経路

ユーザー本人の LLM サブスクリプション（Google AI Pro / ChatGPT Plus 等）で Synthify の
LLM 呼び出しを動かすための設計。**本番デフォルトは Gemini/Vertex のまま変更しない。**

`codex-provider-migration.md`（Codex 前提の RFC）を一般化し、2026-08 の調査で判明した
Antigravity SDK と配備形態の整理を統合したもの。旧 RFC の Phase 構成・セキュリティ要件・
評価ゲートはそのまま引き継ぐ。

## Status

- **Priority**: P3 / architecture proposal
- **Decision**: Gemini は本番デフォルトのまま。provider boundary の背後に local provider を
  追加し、まず個人/ローカル PoC で検証する。
- **Target**: process-tool の LLM client を先に差し替え、ADK agent runtime の置換は別途判断。

## Product boundary

**Provider 認証は Synthify 認証ではない。** ここは旧 RFC の結論をそのまま維持する。

- Firebase が Synthify のユーザー・workspace・認可・アカウント所有の正本。
- ChatGPT / Google の provider 認証は、**認証済み Synthify ユーザーに紐づくオプション接続**。
- provider セッションを Synthify ユーザー間で共有してはならない。
- provider の認証情報と runtime state をブラウザに露出させてはならない。

| Product shape | Provider model | Decision |
| --- | --- | --- |
| Synthify Personal / local | 本人のサブスクセッション | **Supported PoC target** |
| Internal single-user deployment | 隔離された user state | Possible after security review |
| Public multi-user Synthify Cloud | OpenAI API or Gemini/Vertex (managed) | **Production target** |
| 運営者の 1 アカウントを全ユーザーで共有 | 共有セッション | **Prohibited design** |

最後の行が本設計の起点。以下はその根拠。

## なぜサーバーサイドで共有できないか

調査の結論。いずれも実装の工夫では越えられない。

### 1. 規約（個人アカウント）

OpenAI の利用規約はアカウント認証情報の共有を明示的に禁止しており「1サブスク = 1ユーザー」。
複数 IP からの同一アカウント同時アクセスは不正検知の対象で、停止・BAN のリスクがある。
Google 側も個人アカウント OAuth に紐づく。

ユーザーごとに認証を分離する構成（`CODEX_HOME` 分離等）なら規約上は適合するが、次の 2, 3 が残る。

### 2. 常駐プロセス

- `codex app-server` は常駐する JSON-RPC サーバー。
- `google-antigravity` SDK は **Go 製ハーネスバイナリを同梱**し、`LocalConnection` が
  WebSocket でそれと通信する。実装されている接続戦略は `LocalConnection` のみ。

Cloud Run はリクエスト単位でスケールし、**どのリクエストがどのインスタンスに着地するか
制御できない**。ユーザーごとの常駐プロセスとは噛み合わず、GKE 移行 + プロセスライフサイクル
管理の自前実装が必要になる。

### 3. ハーネスの階層（agent runtime を置き換える場合のみ）

Codex も Antigravity も**モデルではなくエージェント**で、agent ループ・tool 実行・approval を
内蔵している。Synthify も
[agents/orchestrator.go](../../apps/worker/pkg/worker/agents/orchestrator.go) で Google ADK
(`llmagent` / `runner` / `session`) の上に自前ハーネスを持つ。

```
[Synthify のハーネス]        [Codex / Antigravity]
      ↓                            ↓
   model.LLM 抽象               モデル(内蔵)
      ↓                            ↓
   Gemini(Vertex)               GPT / Gemini / Claude
```

両者は**同じ高さ**にあり、下に差し込めない。したがって「共通フレームワークに移して両方
載せる」ことはできない。

**ただしこれは agent runtime の話に限る。** process tool が依存しているのは
[llm/client.go](../../apps/worker/pkg/worker/llm/client.go) の小さな `Client` interface
（`GenerateStructured` / `GenerateText` の 2 メソッドのみ）であり、**ここは bounded な
provider 差し替えで済む**。この 2 つを混同しないこと。旧 RFC の Phase 分割はこの区別に
基づいている。

## 配備形態

3 つある。**(b) を第一候補とする。**

### (a) worker 内 provider seam

worker が provider を起動し、`llm.Client` として使う。旧 RFC の Phase 1-3 が想定していた形。

- `LLM_PROVIDER=codex` はローカル/個人 runtime でのみ有効。**production/staging では fail closed**。
- Cloud Run 上では上記 2 の理由で成立しないため、**ローカル配備の Synthify に限られる**。

### (b) ブラウザ + ローカル AI サーバー ← 推奨

既存の web（ブラウザ）はそのまま。LLM 呼び出しだけをユーザーの端末で動かす。

```
ブラウザ（既存の Synthify web、変更なし）
   ↓ fetch → http://localhost:PORT
ローカル AI サーバー（ユーザーが起動）
   ↓ Antigravity SDK / Codex app-server
サブスク枠
```

**Synthify 本体にほぼ変更が要らない**のが決定的な利点。

- フロントの分岐は「ローカルサーバーが居れば使う」だけ。
- Cloud Run / ADK / GCS FUSE は一切変更なし。
- ネイティブ UI 化（Tauri 等）が不要なので、**OS 別 UI ビルド・Firebase Auth の loopback
  対応・tree のローカル正本化がすべて不要**になる。ブラウザは今まで通り API を叩く。

技術的な確認事項:

| 項目 | 内容 |
|---|---|
| CORS | ローカルサーバーが `Access-Control-Allow-Origin` を返す。自前サーバーなので設定するだけ |
| Mixed Content | Chrome/Edge は `http://localhost` を potentially trustworthy origin として扱い例外。**Safari は要実測** |
| ポート衝突 | 固定ポートに拘らず、複数候補を順に試すか設定可能にする |
| 未起動時 | ブラウザ側で検知し、起動を促す or サーバー版にフォールバック |

### (c) 完全ローカル配備

Synthify 一式をローカルで動かす。(a) の前提。個人利用としては成立するが、本設計の主眼ではない。

## Provider 選定

| | Antigravity | Codex |
|---|---|---|
| モデル | Gemini + **Claude Sonnet 4.6 / Opus 4.6** + GPT-OSS-120b | GPT 系のみ |
| SDK | Python 公式 `google-antigravity` | JSON-RPC / TS SDK |
| 用途 | 汎用エージェント | コーディング特化 |
| 従量課金の遮断 | **AI Credit Overages を `Never`** に設定可 | 未確認 |
| 成熟度 | v0.1.12 (alpha, 2026-08-13) | app-server の WebSocket は experimental |

**Antigravity を第一候補とする。**

1. 1 経路で Gemini と Claude の両方が使える。
2. Synthify は Gemini ベースなので
   [prompts/templates/](../../apps/worker/pkg/worker/prompts/templates/) の資産が活きる。
3. overage を `Never` にできる = 「従量課金は使わない」要件に直接効く。

`LLM_PROVIDER=gemini|codex|antigravity` として旧 RFC の provider seam をそのまま拡張する。
**最初から複数 provider を実装しない**。alpha の SDK を 2 つ同時に相手にしない。

### 参考: Gemini CLI の前例

Gemini CLI は 2026-06-18 に個人向けサブスク経路（Login with Google）を終了し、Antigravity へ
移行した。有料 API キー経路は継続。**サブスク枠をプログラムから使う経路は提供側の方針で
閉じられうる**ことの実例であり、本設計を「主」に据えてはならない根拠になる。

## 配布と更新

(b) のローカル AI サーバーの配布方法。

`google-antigravity` は **pure-python ではなくプラットフォーム別 wheel**
（macOS arm64 / Linux x86_64・aarch64 / Windows amd64・arm64、各 32〜39MB）で、Go ハーネスを
同梱している。**元々 OS 別ビルドが前提**なので、単一バイナリ化しても構造は変わらない。

| 方式 | ユーザー要件 | 署名 | 更新 |
|---|---|---|---|
| **pipx** | Python + pipx | 不要 | `pipx upgrade` |
| **単一バイナリ** (PyInstaller) | **なし** | **要る** | 起動時チェック + 手動 DL |

**両方出す**のが良い。手間はほぼ変わらない。

- GitHub Releases → 単一バイナリ（一般ユーザー向け）
- PyPI → `pipx install`（開発者向け）

**署名について**: バイナリにすると OS の実行ファイル検証の対象になる。未署名だと macOS は
Gatekeeper、Windows は SmartScreen が警告を出す。対象が AI Pro 契約者（開発者寄り）なら、
回避手順を案内して**未署名で始めるのが現実的**。後から署名を足しても配布形態は変わらない。

**更新**: 自動更新は実装しない。起動時にバージョンを確認し、新しければ通知して DL URL を
出す程度で足りる（ローカルサーバーは常時起動する性質のものではない）。

**API 互換性**: サーバー API が変わったとき古いローカルサーバーが壊れないよう、
**サポート最低バージョンをサーバーから返す**仕組みを用意する。

## Phase

旧 RFC の構成を引き継ぐ。Phase 1 が起点。

### Phase 1: provider seam（振る舞い変更なし）

worker 起動時に provider を選択できるようにする。Gemini がデフォルト。

```
process tools
    |
    v
llm.Client
    |-- GeminiClient        (production default)
    `-- LocalProviderClient (local PoC only)
```

```
LLM_PROVIDER=gemini|antigravity|codex
LOCAL_PROVIDER_ENDPOINT=http://127.0.0.1:PORT
```

`gemini` 以外は**ローカル/個人 runtime として明示された場合のみ有効**。production / staging は
managed provider を要求し続ける（fail closed）。

### Phase 2: ローカル AI サーバー

- HTTP + CORS の薄いサーバー
- Antigravity SDK 呼び出し（`GenerateText` / `GenerateStructured` 相当）
- structured output の検証
- provider エラーの正規化
- プロセス終了時のクリーンアップ

### Phase 3: source files

現行の Gemini client は Files API にアップロードするが、ローカル provider には
**job スコープのローカル作業ディレクトリ**を渡す。

```
GCS source → job 固有の一時ディレクトリ → 正規化/展開 → provider 作業ディレクトリ
          → structured response → 確実なクリーンアップ
```

要件: job ごとに 1 ディレクトリ / ユーザー・workspace 間で再利用しない / 初回 PoC は read-only /
許可されたルートパスのみ / 成功・失敗・キャンセルいずれでも確実に削除 / サイズとファイル数の上限。

### Phase 4: provider 接続

Firebase ログインとは**別の** provider 接続状態を持つ。

```
Firebase session → Synthify アカウントと workspace 認可
provider 接続    → このユーザー/デバイスで provider が使えるか
```

トークンと認証ファイルは**隔離されたローカル runtime に留め**、Firestore / PostgreSQL / ログ /
ブラウザストレージに複写しない。Synthify が保持するのは最小限の接続メタデータのみ。

### Phase 5: 評価ゲート

本番デフォルトを変える前に、代表的な fixture で Gemini と比較する。既存 eval runner を使う。

- structured JSON の妥当性
- knowledge-tree の品質
- 引用/出典の grounding
- tool 選択の正確さ
- レイテンシとタイムアウト挙動
- retry 挙動
- 大きなドキュメントの扱い
- quota / rate-limit 挙動
- 確実なクリーンアップ

provider・model・fixture・所要時間・入出力サイズ・検証結果・失敗分類を記録する。

**`ContentNode[]` の structured output はモデルごとに品質が変わりうる**ため、Claude と Gemini
の両方で検証する（[paper-native-llm-interaction.md](paper-native-llm-interaction.md) 参照）。

### Phase 6: agent runtime の判断

process-tool PoC の後、2 択:

1. **ADK を維持**し、local provider は process-tool 呼び出しにのみ使う。
2. **ADK を置換**し、provider の thread/turn orchestration に移行、Synthify のツールを MCP 等の
   安定した境界で公開する。

2 は別 migration。`model.LLM` 初期化 / `llmagent` orchestration / ADK tool adapter /
before-after model callback / before-after tool callback / agent metering callback /
キャンセルと checkpoint 挙動、すべての置換を伴う。

## Usage と billing

既存の課金は provider 報告の input/output token を前提に、要求元アカウントへ費用を帰属させる。
**サブスク枠はこのモデルに乗らない。**

個人 PoC:

- usage は**運用テレメトリとして記録し、金銭的な課金にしない**
- provider と quota/rate-limit 状態を Stripe のコストとは別に露出する
- サブスク利用分を顧客請求に加算しない

Synthify Cloud:

- managed API provider を使い、サーバー側で明示的に計測する
- 現行のアカウント帰属と予算強制のセマンティクスを維持する

**利用実態の可視化**: ローカル provider はコストが発生しないぶん、利用状況も見えなくなる。
匿名の利用統計を送る場合は、**対話内容を含めない形**とプライバシーの説明が要る。

## セキュリティ要件

旧 RFC から引き継ぐ。

- provider サーバーを信頼できないネットワークに直接公開しない。
- provider の認証情報・認証ファイルをブラウザに送らない。
- ユーザー/デバイス ID ごとに隔離された state ディレクトリを 1 つ。
- job ごとに隔離された作業ディレクトリを 1 つ。
- 既定は read-only サンドボックス。
- write / shell 権限の有効化には明示的な承認を要求する。
- ローカルデバッグを明示的に有効化しない限り、prompt / 出力 / ファイルパス / 認証メタデータを
  ログから除去する。
- プロセス数上限・リクエスト期限・キャンセル・孤児プロセスの回収を入れる。
- ある Firebase ユーザーが他ユーザーの provider 接続を選べないようにする。

## 最初の実装 PR

意図的に小さくする。

1. `LLM_PROVIDER` のパース（既定 `gemini`）
2. provider 初期化を ADK 初期化から分離
3. `llm.Client` を実装する `LocalProviderClient` を追加
4. text と structured 生成のみ対応
5. source files は当面「未対応」の明示エラー、または job ディレクトリへの staging が
   整ってから対応
6. fake transport による unit test
7. 環境変数でガードしたローカル限定の integration test
8. **production の Terraform と既定値は Gemini のまま**

## PoC の受け入れ条件

- 既存の Gemini テストと本番起動挙動が変わらない。
- ローカルで text 生成 fixture が 1 本通る。
- structured 生成が schema 妥当な JSON を返す。
- キャンセルで対応する turn とサブプロセスが停止する。
- 通常ログに認証情報・ソース内容が出ない。
- 並行 job が互いの作業ディレクトリと provider state を読めない。
- provider エラーが生の JSON-RPC ペイロードではなく型付き worker エラーとして表面化する。
- usage テレメトリが provider を区別し、サブスク経由の実行で Stripe 課金が発生しない。

## Rollout

1. provider boundary の refactor をマージ（振る舞い変更なし）。
2. `LLM_PROVIDER` 経由でローカル PoC を投入。
3. eval fixture を実行し結果を記録。
4. 個人機能に留めるか、API バックエンドのクラウド provider にするか、ADK を置換するかを判断。
5. その後に初めてデプロイ構成と製品 UI を更新。

## Open Questions

- **`conversation_id` の resume 範囲**: プロセス再起動をまたげるか、履歴がローカルか
  Google サーバー側か。公式ドキュメントで確認できず、実装前に SDK ソースまたは実挙動で確かめる。
- **Safari の Mixed Content 挙動**: `http://localhost` への fetch が通るか要実測。
- **サブスク枠を使い切ったときの挙動**: overage を `Never` にした場合の SDK のエラー形と、
  UI での見せ方（サーバー版へのフォールバックを提案するか）。
- **モデル選択を誰がするか**: ユーザーが選ぶか、用途ごとに固定するか。
- **プロンプトの共有**: サーバー版の [prompts/templates/](../../apps/worker/pkg/worker/prompts/templates/)
  は Go 側。ローカル provider（Python）から同じテンプレートを使うなら共有の仕組みが要る。

## 関連

- `codex-provider-migration.md` — 本ドキュメントの前身（Codex 前提）。統合済み
- [../contracts/chat-turn-contract.md](../contracts/chat-turn-contract.md) — 対話の契約。
  `content: ContentNode[]` は provider に依存しない
- [paper-native-llm-interaction.md](paper-native-llm-interaction.md) — `ContentNode[]` と
  `Command` の構想。structured output の品質が provider 選定に効く
- [usage-based-billing.md](usage-based-billing.md) — サーバー版のコスト転嫁。本設計と補完関係
