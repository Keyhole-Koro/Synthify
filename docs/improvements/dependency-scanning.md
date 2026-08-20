# 依存・設定スキャンの構成

CI に脆弱性スキャンが一切無い状態（`dependabot.yml` なし、CodeQL なし、`govulncheck` なし、`bun audit` なし）を解消するための構成メモ。実装は `.github/dependabot.yml` / `.github/workflows/security.yml` / `.github/workflows/codeql.yml`。

## 構成

| 対象 | ツール | 実行場所 | ゲート |
|---|---|---|---|
| Go 依存 + stdlib | `govulncheck` v1.7.0 | `security.yml` / `govulncheck` | **blocking** |
| npm 依存 | `bun audit` | `security.yml` / `bun_audit` | report-only（run summary） |
| Terraform / Dockerfile | `trivy config` (HIGH,CRITICAL) | `security.yml` / `iac` | report-only（Security タブ） |
| Go / TS のコード | CodeQL | `codeql.yml` | report-only（Security タブ） |
| バージョン更新 PR | Dependabot (`gomod` / `bun` / `github-actions` / `docker` / `terraform`) | — | — |

`security.yml` と `codeql.yml` はどちらも weekly の `schedule` を持つ。差分トリガだけでは「コードは変わっていないが advisory が新しく出た」ケースを一生検知できないので、実際に効くのは schedule の方で、PR トリガは新規の露出をマージさせないための補助という位置づけ。

## Snyk を採らなかった理由

1. **bun lockfile を読めない。** フロントの依存は top-level の `bun.lock` だけで解決されていて `package-lock.json` は存在しない。商用 SCA の JS サポートは npm/yarn/pnpm 前提なので、導入するには CI で `package-lock.json` を合成することになる。それは bun が実際に入れるツリーとは別物なので、**出荷していないバージョンの脆弱性を報告し、出荷しているものを見逃す**。依存の大半（`next` / `react` / `firebase` / `recharts` …）がここに乗っているので致命的。`bun audit` は `bun.lock` を直接読むため、このリポジトリで npm advisory の正確な出典はこれしかない。
2. **Go は `govulncheck` の方が精度が高い。** 呼び出し到達性で絞るので「`go.sum` に入っているだけで実行パスに無い CVE」を出さない。manifest マッチングのスキャナはここでノイズになる。無料・公式という以前に、単純に出力の質が違う。
3. **リポジトリが public。** CodeQL / Dependabot alerts / secret scanning が全部無料で使える。private なら CodeQL は GHAS 課金なので商用 SAST を比較する意味があったが、public だとその差別化が消える。
4. **運用コスト。** アカウント管理、`SNYK_TOKEN` の secret 管理、無料枠のテスト回数上限、PR コメントのノイズ。この規模でダッシュボードに払う価値が見合わない。

再検討する価値があるのは、ライセンスコンプライアンスが要件化したとき、リポジトリが複数に増えて横断ダッシュボードが欲しくなったとき、あるいは private 化して CodeQL が使えなくなったとき。

## `govulncheck` の初回結果（解消済み）

導入直後の初回実行で **到達可能な脆弱性 20 件** を検出した。内訳:

- **17 件が Go 標準ライブラリ** — `go.mod` が `go 1.25.8` を固定していたため。修正版は 1.25.9〜1.25.13 に分散していたので、`go 1.25.13` への引き上げで全部消える
- `google.golang.org/grpc` v1.81.1 → v1.82.1（GO-2026-6061）
- `golang.org/x/text` v0.37.0 → v0.39.0（GO-2026-5970）

実害の観点で読み直した結果:

- **GO-2026-6089（h2c チェック時に `ReadHeaderTimeout` が効かない）** — 20 件中これだけが具体的なデータ経路として成立する。worker が `http.ListenAndServe` を素で呼んでいて（`apps/worker/cmd/server/main.go:101`）`http.Server` を組んでいないため、`ReadHeaderTimeout` がそもそも未設定。リッスンしているソケットに直接効く
- **crypto/tls 系 3 件**（GO-2026-6090 / 5856 / 4870）— Cloud Run が前段で TLS 終端するのでコンテナ内の Go サーバは平文 HTTP を話す。トレースは外向きクライアント接続由来で、露出は低い
- **grpc GO-2026-6061** — xDS RBAC と HTTP/2 *サーバ*実装の脆弱性。こちらは gRPC をクライアント（New Relic / Cloud Tasks）としてしか使っていない
- **html/template 系 4 件** — 到達トレースが `http.ListenAndServe` や `fmt.Fprintf` 経由の間接的なもので、テンプレートに攻撃者入力を流している箇所は無い

### 「到達可能」の読み方（重要）

govulncheck の到達性判定は**静的呼び出しグラフ**であって、データフロー解析ではない。インターフェース越しのメソッド呼び出しは、**バイナリ中に存在する全実装**に解決される。そのため実際には繋がっていない辺が生える。

このリポジトリで実際に踏んだ例が GO-2026-5970（x/text の `norm.Iter` 無限ループ）。レポートは

```
io.repairEncoding calls transform.Bytes, which eventually calls norm.Form.Transform
```

というトレースを出すが、`transform.Bytes(t Transformer, b []byte)` の第 1 引数はインターフェースで、`repairEncoding` が実際に渡しているのは `japanese.ShiftJIS` 等のデコーダである。`x/text/encoding/**` は `unicode/norm` を import していない（テストを除く）ので、この経路で発火することはない。`norm.Form` がバイナリに居るのは `x/net/idna`（`net/http` のホスト名正規化）が使っているからで、グラフはそちらと繋がっただけ。

`encoding/xml` の GO-2026-6088（`sql.Rows.Next` → `xml.Unmarshal`）も同種で、XML カラムは使っていない。

manifest マッチングのスキャナよりはるかに精度が高いのは確かだが、**トレースを実際のデータ経路として読むと過大評価する**。修正するかどうかの判断には使えるが、深刻度の見積もりには読み解きが要る。

### 運用上の注意

`go.mod` の go directive を patch version まで固定しているので、Go の patch release が出るたびにここが遅れると govulncheck が赤くなる。Dockerfile 側は `golang:1.25-bookworm` で patch に追随するため、手で追従が必要なのは `go.mod` の 1 行だけ。Dependabot の `gomod` エントリは go directive までは面倒を見ない。

## `bun audit` を blocking にしていない理由

導入時点で 77 件（critical 2 / high 41 / moderate 27 / low 7）の advisory がある。このうち以下はこのリポジトリの変更では解消できない:

- **`sharp` < 0.35.0** — `next` が optional dependency として `^0.34.5` を固定している。next 側が上げるまで動かせない。
- **`vitest` 3.2.4 (critical)** — vendored submodule `apps/web/vender/paper-in-paper` の devDependencies 由来。上流の問題。

作者が直せない理由で毎回 PR が赤くなる状態でゲートを入れても、`continue-on-error` を足されて終わるだけなので、run summary へのレポートに留めている。

一方で bump で消えるものも多い:

- `next` 16.2.4 → 16.2.5 以降（advisory 約 11 件 + `next/postcss` 8.4.31 が同時に解消。`package.json` の `^16.2.2` の範囲内なので `bun update next` で足りる）
- `protobufjs` 7.5.6 → 7.6.0 超（`firebase` / `firebase-admin` 経由）
- `@newrelic/browser-agent` 経由の `postcss`

**blocking に切り替える条件**: 上記の bump を済ませ、残りが「上流待ちの既知 2 件」だけになった時点で `bun audit --audit-level=high` を `run` に戻してゲート化する。上流待ちの分は bun 側に ignore の仕組みが入るか、上流が上がるかのどちらかで畳む。

なお Dependabot の bun 対応は **version updates のみ**で security updates は未実装なので、advisory 起点の自動 PR は飛んでこない。そのギャップを埋めているのが `bun audit` ジョブ。

## マージ前に確認すること

- **CodeQL の default setup が有効だとぶつかる。** リポジトリ設定 (Settings → Code security → Code scanning) で default setup が有効になっている場合、`codeql.yml`（advanced setup）の実行は `CodeQL analyses from advanced configurations cannot be processed when the default setup is enabled` で失敗する。有効なら先に無効化する。
- `security.yml` / `codeql.yml` は `pull_request` と `push: main` にしか反応しないので、feature ブランチへの push だけでは一度も走らない。初回の検証は PR を開いた時点で行われる。
