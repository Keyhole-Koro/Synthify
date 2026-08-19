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
