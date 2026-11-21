# 依存・設定スキャンの構成

CI に脆弱性スキャンが一切無い状態（`dependabot.yml` なし、CodeQL なし、`govulncheck` なし、`bun audit` なし）を解消するための構成メモ。実装は `.github/dependabot.yml` / `.github/workflows/security.yml` / `.github/workflows/codeql.yml` / `scripts/govulncheck-gate.sh`。

## 構成

| 対象 | ツール | 実行場所 | ゲート |
|---|---|---|---|
| Go 依存 + stdlib | `govulncheck` v1.10.0 | `security.yml` / `govulncheck` | **blocking**（PR はその PR が増やした分だけ） |
| npm 依存 | `bun audit`（ルートの `bun.lock` と `apps/tree-viewer/bun.lock`） | `security.yml` / `bun_audit` | report-only（run summary） |
| Terraform / Dockerfile | `trivy config` (HIGH,CRITICAL) | `security.yml` / `iac` | report-only（Security タブ） |
| Go / TS のコード | CodeQL | `codeql.yml` | report-only（Security タブ。PR では新しいアラートだけが PR のチェックに出る） |
| バージョン更新 PR | Dependabot (`gomod` / `bun` / `github-actions` / `docker` / `terraform`) | — | — |

`security.yml` と `codeql.yml` はどちらも weekly の `schedule` を持つ。差分トリガだけでは「コードは変わっていないが advisory が新しく出た」ケースを一生検知できないので、実際に効くのは schedule の方で、PR トリガは新規の露出をマージさせないための補助という位置づけ。

必要な secret は失敗通知の `DISCORD_WEBHOOK_URL` だけで、リポジトリに既にある。Snyk などのトークンは使わない。

### govulncheck のゲート（`scripts/govulncheck-gate.sh`）

- **PR**: PR の base コミットも同じ toolchain でスキャンし、「HEAD で到達可能」かつ「base では到達可能でなかった」脆弱性があるときだけ落とす。base に元からある指摘で PR が赤くなることはなく、直す PR は緑になる。脆弱な依存を足す PR、脆弱な関数を新しく呼ぶ PR は落ちる。PR のトリガは依存ファイルに加えて `*.go` も含む（コードの変更で到達性が変わるため）。
- **schedule / main への push / 手動実行**: 到達可能な脆弱性が 1 件でもあれば落とし、Discord に通知する。新しく公開された advisory はここで気づく。
- どの場合も、HEAD の到達可能な脆弱性の一覧と govulncheck のテキスト出力を run summary に出す。

### Actions の固定

サードパーティの Action はリリースのコミット SHA で固定し、コメントにリリース名を書く（`uses: actions/checkout@<sha> # v4.4.0`）。更新は Dependabot の `github-actions` が PR を出す。

`aquasecurity/trivy-action` は 2026-03 にタグを書き換えられる事件があった（GHSA-69fq-xp46-6x23）。使っている v0.36.0 はその後の immutable release で、SHA 固定に加えて Trivy 本体の版（`version: v0.70.0`）も明示している。

## Snyk を採らなかった理由

1. **bun lockfile を読めない。** フロントの依存は `bun.lock` だけで解決されていて `package-lock.json` は存在しない。商用 SCA の JS サポートは npm/yarn/pnpm 前提なので、導入するには CI で `package-lock.json` を合成することになる。それは bun が実際に入れるツリーとは別物なので、**出荷していないバージョンの脆弱性を報告し、出荷しているものを見逃す**。依存の大半（`next` / `react` / `firebase` / `recharts` …）がここに乗っているので致命的。`bun audit` は `bun.lock` を直接読むため、このリポジトリで npm advisory の正確な出典はこれしかない。
2. **Go は `govulncheck` の方が精度が高い。** 呼び出し到達性で絞るので「`go.sum` に入っているだけで実行パスに無い CVE」を出さない。manifest マッチングのスキャナはここでノイズになる。無料・公式という以前に、単純に出力の質が違う。
3. **リポジトリが public。** CodeQL / Dependabot alerts / secret scanning が全部無料で使える。private なら CodeQL は GHAS 課金なので商用 SAST を比較する意味があったが、public だとその差別化が消える。
4. **運用コスト。** アカウント管理、`SNYK_TOKEN` の secret 管理、無料枠のテスト回数上限、PR コメントのノイズ。この規模でダッシュボードに払う価値が見合わない。

再検討する価値があるのは、ライセンスコンプライアンスが要件化したとき、リポジトリが複数に増えて横断ダッシュボードが欲しくなったとき、あるいは private 化して CodeQL が使えなくなったとき。

## Go の指摘と対応

### 2026-08（導入時）

導入直後の初回実行で **到達可能な脆弱性 20 件** を検出した。17 件が Go 標準ライブラリ（`go 1.25.8` 固定のため）、残りが `google.golang.org/grpc`（GO-2026-6061）と `golang.org/x/text`（GO-2026-5970）。

### 2026-11-21（このブランチを main に合わせ直した時点）

- **Go 1.25 はサポートが終わっている。** Go 1.27 が出たので、セキュリティ修正が出るのは 1.27 と 1.26 だけになった（1.25 は 1.25.14 が最後）。1.25 のままだと標準ライブラリの新しい脆弱性は直らず、govulncheck が赤くなり続ける。`go` directive を **1.26.11** に上げ、`apps/api` / `apps/worker` / `apps/eval` の Dockerfile と `compose.yaml` のビルド用イメージを `golang:1.26-bookworm` にした。
- Go 1.26.11 で到達可能だったのは依存の 8 件で、すべて修正版への引き上げで消えた:
  - `golang.org/x/text` → v0.43.0（GO-2026-6629。GO-2026-5970 も含む）
  - `golang.org/x/net` → v0.62.0（GO-2026-6603 / 6610 / 6611 / 6612 / 6617。到達性の無い GO-2026-6860 / 6866 も）
  - `go.opentelemetry.io/otel/sdk` ほか otel 一式 → v1.45.0（GO-2026-6505）
  - `google.golang.org/grpc` → v1.83.2（GO-2026-6348。パッケージ単位で出ていた GO-2026-6443 も）
- 残っているのは `golang.org/x/crypto` の GO-2026-5932 だけで、モジュール単位の指摘（使っているパッケージには無い）。修正はタグの無い pseudo-version にしか入っていないので、リリースを待つ。
- `go test ./...`（40 パッケージ）、`go vet ./...`、`go build ./...` は Go 1.26.11 で通る。

### HTTP サーバのタイムアウト（GO-2026-6089 の関連）

導入時にこのブランチで api / worker の `http.ListenAndServe` を `http.Server{ReadHeaderTimeout: ...}` に置き換えていたが、同じ変更をセキュリティ修正の PR #55 が共通パッケージ `internal/platform/httpserver`（テスト付き）で入れるので、こちらからは外した。両方に残すと、後からマージする方が同じ行で衝突する。

### 「到達可能」の読み方（重要）

govulncheck の到達性判定は**静的呼び出しグラフ**であって、データフロー解析ではない。インターフェース越しのメソッド呼び出しは、**バイナリ中に存在する全実装**に解決される。そのため実際には繋がっていない辺が生える。

このリポジトリで実際に踏んだ例が GO-2026-5970（x/text の `norm.Iter` 無限ループ）。レポートは

```
io.repairEncoding calls transform.Bytes, which eventually calls norm.Form.Transform
```

というトレースを出すが、`transform.Bytes(t Transformer, b []byte)` の第 1 引数はインターフェースで、`repairEncoding` が実際に渡しているのは `japanese.ShiftJIS` 等のデコーダである。`x/text/encoding/**` は `unicode/norm` を import していない（テストを除く）ので、この経路で発火することはない。`norm.Form` がバイナリに居るのは `x/net/idna`（`net/http` のホスト名正規化）が使っているからで、グラフはそちらと繋がっただけ。

manifest マッチングのスキャナよりはるかに精度が高いのは確かだが、**トレースを実際のデータ経路として読むと過大評価する**。修正するかどうかの判断には使えるが、深刻度の見積もりには読み解きが要る。

### 運用上の注意

- `go.mod` の go directive を patch version まで固定しているので、Go の patch release が出るたびにここが遅れると、schedule の govulncheck が赤くなる。Dockerfile と `compose.yaml` は `golang:1.26-bookworm` で patch に追随するため、patch で手を動かすのは `go.mod` の 1 行だけ。Dependabot の `gomod` エントリは go directive までは面倒を見ない。
- minor（1.26 → 1.27）を上げるときは、`go.mod` と一緒に Dockerfile 3 つと `compose.yaml` の 2 か所のイメージも上げる。公式イメージは `GOTOOLCHAIN=local` なので、イメージの Go が directive より古いとビルドが止まる。Go 1.26 のサポートは Go 1.28 のリリース（2027-02 の見込み）までなので、それまでに 1.27 へ移る。
- govulncheck v1.9.0 以降は Go 1.26 以上でしか動かない。

## npm の指摘

### 2026-11-21 の対応

`next` を `^16.4.6` に上げた（apps/web と apps/monitor/ui）。16.2 系には 16.4.1 / 16.4.5 でしか直っていない advisory（critical を含む）があるため、16.4 系にした。あわせて `next` の optional dependency の `sharp` が 0.35 系になり、`next/postcss` も新しくなった。`bun audit` は 151 件（critical 9 / high 74 / moderate 68）から **111 件（critical 4 / high 57 / moderate 50）** に減り、`next` と `sharp` の advisory は 0 件になった。

`next build` の型チェックが 16.4 からテストファイルも見るようになり、`workspaceSnapshotCache.test.ts` の BigInt リテラル（target ES2017 では使えない）で落ちたので、`BigInt(n)` に書き換えた。

なお PR #51 は apps/web と apps/monitor/ui を Vite に移すので、マージされれば `next` 自体が依存から消える。それまでの本番（静的書き出しの web と、Cloud Run で Next サーバとして動く monitor）を守るための引き上げ。

### 残り（111 件）と出どころ

| 出どころ | 主な advisory | 本番に届くか |
|---|---|---|
| `vitest`（apps/web と、vendored の paper-in-paper の devDependencies） | `tinypool`（critical 2）、`vitest` UI（critical）、`@vitest/mocker` | 届かない（テストだけ） |
| `eslint` / `eslint-plugin-react-hooks` | `minimatch`、`brace-expansion`、`js-yaml`、`browserslist` | 届かない（lint だけ） |
| `jsdom` | `undici`、`form-data` | 届かない（テストだけ） |
| `@vitejs/plugin-react` | `vite` 7.x | 届かない（テストだけ） |
| `firebase`（web と monitor のブラウザ側） | `websocket-driver`（critical）、`@grpc/grpc-js` | どちらも Node 用の経路で、ブラウザのバンドルには入らない見込み |
| `firebase-admin`（monitor のサーバ側、web の開発用） | `node-forge`、`protobufjs`、`uuid`、`fast-xml-builder`、`@fastify/busboy` | monitor のサーバに入る。`firebase-admin` の更新待ち |
| `@tailwindcss/postcss` | `postcss`、`nanoid`、`source-map-js` | 届かない（ビルド時だけ） |
| `@newrelic/browser-agent` | `fflate` | web のブラウザに入る |
| `zod`（monitor） | `zod` < 4.6.6 | monitor のサーバに入る |
| `next`（monitor） | `baseline-browser-mapping` | ビルド時のデータだけ |

次に上げるなら、`firebase-admin` と `zod`（monitor のサーバ）、`@newrelic/browser-agent`（web のブラウザ）から。どれも各パッケージの範囲内の更新で済むかを、テストと一緒に確かめてから入れる。PR #51 がマージされた後の方が、衝突が少ない。

### `bun audit` を blocking にしていない理由

上の残りの大半は、このリポジトリの変更だけでは消せない（上流の依存の更新待ち、vendored submodule の devDependencies）。作者が直せない理由で毎回 PR が赤くなる状態でゲートを入れても、`continue-on-error` を足されて終わるだけなので、run summary へのレポートに留めている。

**blocking に切り替える条件**: 残りが上流待ちの既知の分だけになった時点で、govulncheck と同じく「base に無い advisory だけで落とす」形でゲート化する。

なお Dependabot の bun 対応は **version updates のみ**で security updates は未実装なので、advisory 起点の自動 PR は飛んでこない。そのギャップを埋めているのが `bun audit` ジョブ。

## マージ前・マージ後に確認すること

- **CodeQL の default setup が有効だとぶつかる。** リポジトリ設定 (Settings → Code security → Code scanning) で default setup が有効になっている場合、`codeql.yml`（advanced setup）の実行は `CodeQL analyses from advanced configurations cannot be processed when the default setup is enabled` で失敗する。有効なら先に無効化する。
- **Dependabot alerts と security updates はリポジトリ設定で有効にする。** `dependabot.yml` は version updates だけを決める。alerts と Go などの security updates は Settings → Code security で有効にしないと動かない。
- Dependabot の PR には Actions の secret が渡らない。失敗通知を Dependabot の PR でも送りたいなら、`DISCORD_WEBHOOK_URL` を Dependabot secrets にも登録する。
- `security.yml` / `codeql.yml` は `pull_request` と `push: main` にしか反応しないので、feature ブランチへの push だけでは一度も走らない。初回の検証は PR を開いた時点で行われる。
