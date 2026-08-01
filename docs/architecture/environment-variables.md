# 環境変数・機密情報の管理方針

設定値がどこで決まり、誰が所有し、欠けたときに何が起きるかを定義します。

このプロジェクトでは同じ値が複数箇所に書かれて静かにずれる事故が繰り返し起きています
（CI とローカルで Firebase エミュレータ設定がずれてバンドルが実 Firebase を向いた、
monitor の DSN 未投入がアプリではなくデプロイゲートで偶然止まった、など）。
本ドキュメントはその再発を防ぐための取り決めです。

---

## 1. 環境は 4 つ、それぞれに所有者がいる

| 環境 | 実体 | 設定の所有者 |
| :--- | :--- | :--- |
| **local** | `docker compose up` | `compose.yaml`（既定値）+ `.env`（個人の上書き） |
| **e2e / CI** | GitHub Actions + 同じ compose | `compose.yaml` から**導出**する。CI は再記述しない |
| **stage** | GCP `synthify-stage-491705` | Terraform（`tfvars` / `locals.tf`）+ Secret Manager |
| **prod** | GCP `synthify-491705` | 同上 |

**原則 1: 値の所有者は 1 箇所。** 他所は所有者から導出するか、参照するだけ。

e2e が compose から導出する具体例が `scripts/compose-browser-env.sh` です。
`NEXT_PUBLIC_API_BASE_URL` などはポート変数を合成しているため、
そもそも静的にコピーしても正しくなり得ません。

**原則 2: 秘密情報は git に入らない。**
local は `.env`（gitignore 済み）、CI は**偽物とわかるダミー**、
stage/prod は Secret Manager。

**原則 3: 非秘密の stage/prod 設定は `tfvars` に置く。**
GitHub Environment の Variables は「git に置けない・置きたくない」値だけに使います。
同じ事実が `tfvars` と GitHub Variables の両方にあると、
リポジトリを読んでも実際の値が分からなくなります。

---

## 2. ビルド時に焼き込まれる値と、実行時に読む値

**この区別を間違えると、症状が原因からかけ離れます。**

| 種別 | 例 | 供給する責任を負うのは |
| :--- | :--- | :--- |
| **ビルド時** | `NEXT_PUBLIC_*`（クライアントバンドルへ焼き込み） | **成果物をビルドした主体** |
| **実行時** | それ以外すべて | コンテナ / プロセスを起動した主体 |

`NEXT_PUBLIC_*` は `next build` の時点で文字列としてバンドルに埋め込まれます。
実行時に環境変数を与えても**後から変わりません**。

したがってビルドを行う主体ごとに供給元が決まります。

| ビルドする主体 | 供給元 |
| :--- | :--- |
| compose の frontend（`next dev`） | `compose.yaml` の `environment` |
| CI（e2e 用のビルド） | `scripts/compose-browser-env.sh frontend` |
| `deploy-frontend.yml`（stage/prod） | GitHub Environment Variables |

**原則 4: ビルド成果物に、その環境に属さないものを入れない。**
テスト専用のフックや開発用の分岐は、出荷される成果物に含まれてはいけません。
これはコメントではなく、成果物に対する検査で担保します。

---

## 3. 値が欠けたときの振る舞い（3 階層）

**原則 5: 必須が欠けたら、静かに動かず、落ちる。**

| 階層 | 定義 | 欠けたときの振る舞い | 例 |
| :--- | :--- | :--- | :--- |
| **T1** | 全環境で必須 | **起動時に落とす**（例外なし） | DB DSN、`INTERNAL_WORKER_TOKEN`、Firebase project ID |
| **T2** | 特定環境で必須 | **その環境でのみ落とす** | Stripe キー（prod では必須、local では不要） |
| **T3** | 任意・機能縮退 | **落とさない**。当該機能を無効化して起動する | New Relic 一式、`DEBUG_LOGS` |

### 既定値についての制約

> **既定値を持ってよいのは、その既定値が正しい環境にしか届かないときだけ。**

`postgres://monitor@127.0.0.1:5432/...` のような local 前提の既定値を T1 の変数に置くと、
prod に届いたときに**起動には成功して機能だけが壊れます**。
これは最も発見が遅れる壊れ方です。

local 用の既定値が必要なら、その環境でしか到達できないことをコードで保証してください
（例: `ENV=local` のときだけ既定値を使い、それ以外では欠落を致命とする）。

---

## 4. 格納場所の一覧

| 分類 | 格納場所 | 内容の例 | 管理方法 |
| :--- | :--- | :--- | :--- |
| **デプロイ用機密情報** | GitHub Actions Secrets | WIF プロバイダー ID、サービスアカウント | GitHub の設定画面 |
| **アプリ用機密情報** | Google Secret Manager | DB 接続文字列、Webhook シークレット | Terraform が器を作成し、値は手動投入 |
| **アプリ用一般設定** | Terraform `tfvars` | モデル名、CORS 許可ドメイン、各種 ID | `terraform/tfvars/` でコード管理 |
| **local / e2e の構成** | `compose.yaml` | ポート、エミュレータ endpoint | コード管理。CI はここから導出 |

Secret Manager に投入する値の一覧と、`monitor-database-dsn` の作り方は
[`terraform/README.md`](../../terraform/README.md) にあります。
**本ドキュメントでは再掲しません**（一覧が 2 箇所にあると必ずずれるため）。

---

## 5. 環境ごとの設定手順

### ローカル開発環境

`compose.yaml` が非秘密の既定値をすべて持っているので、**何も設定しなくても起動します**。
秘密情報や個人的な上書きが必要なときだけ `.env` を用意します。

1. （必要なら）`.env.example` を `.env` にコピーし、Stripe キー等を記入
2. `docker compose up`

`.env` は `compose.yaml` の `${VAR:-default}` を上書きするためのもので、
**それ自体が source of truth ではありません**。

### ステージング・本番環境 (GCP)

#### ① Secret Manager への値の投入

Terraform が器を作った後、値を投入します。詳細な一覧とコマンドは
[`terraform/README.md`](../../terraform/README.md) を参照してください。

#### ② Terraform への設定

`terraform/tfvars/<env>.tfvars` に一般設定を記述して `terraform apply`。

#### ③ GitHub Actions の設定

`Settings > Secrets and variables > Actions` で、デプロイに必要なものだけ登録します。

- Secrets: `GCP_WIF_PROVIDER`、`GCP_WIF_SA_EMAIL`
- Variables: フロントエンドのビルド時に焼き込む `NEXT_PUBLIC_*`

---

## 6. 新しい変数を追加するときの手順

触る箇所は**その変数がどの環境に属し、ビルド時か実行時か**で決まります。

### local / e2e で使う非秘密の値

1. `compose.yaml` の該当サービスの `environment` に追加（**ここが所有者**）
2. アプリ側の設定モジュールに、T1 / T2 / T3 のどれかを明示して追加
   - `apps/web/src/config/env.ts`
   - `apps/monitor/ui/src/config.ts`
   - `apps/api/internal/config/config.go`
   - `apps/worker/pkg/worker/config/config.go`
3. `.env.example` に追記（開発者向けの説明として）

CI のワークフローに書き足す必要は**ありません**。`compose.yaml` から導出されます。

### stage / prod の非秘密の値

1. `terraform/environments/variables.tf` に変数を定義
2. `terraform/services/*/main.tf` の `env_vars` で渡す
3. `terraform/tfvars/<env>.tfvars` に値を記述
4. アプリ側の設定モジュールに階層を明示して追加

### 機密情報

1. `terraform/services/platform/main.tf` の secret 一覧に名前を追加
2. `terraform/services/*/main.tf` の `secret_env_vars` に定義を追加
3. `deploy-backend.yml` の secret ゲートの一覧に追加
4. Secret Manager に値を投入（Terraform は器しか作りません）
5. `terraform/README.md` の一覧を更新

### ブラウザに露出する値（`NEXT_PUBLIC_*`）

上記に加えて、**ビルドする主体すべて**に供給されているか確認してください（§2 の表）。
一箇所でも欠けると、その主体が作ったバンドルだけが壊れます。
