# AGENTS.md

Synthify で作業するエージェント（Claude Code、Codex など）が守るルール。作業のたびに読み込まれるので、どの作業にも効くことだけを短く書く。構成と E2E の流し方は [README.md](README.md)、手順の詳細は `docs/` にある。

## 進め方

- 作業の途中で別の問題や、ほかにやるべきことを見つけたら、今の作業には混ぜない。作業の最後に報告する。直すかどうかはオーナーが決める。
- 確かめていないことは「未確認」と書く。監視、通知、時間切れのような仕組みは、設定を読んだだけで「動く」と書かない。実際に鳴らし、届くところまで見る。
- 自分が起動していないプロセスを止めない。オーナーの作業ツリーと、ほかのエージェントの worktree には触らない。

## Git と PR

- 作業の前に `git fetch origin` し、`origin/main` から新しいブランチを作り、自分専用の worktree で始める。手元の main は古いことがあり、オーナーの作業ツリーには作業中の変更がある。

  ```bash
  git fetch origin
  git worktree add -b <type>/<topic> ../Synthify-<topic> origin/main
  cd ../Synthify-<topic> && git submodule update --init   # apps/web をビルドするときに要る
  ```

- 1 つの PR には 1 つの関心事だけを入れる。ついでに見つけた整形や別の修正は、別の PR にする。
- ブランチ名は `種類/内容` にする（例：`fix/api-cross-workspace-checks`）。
- コミットは英語で書く。タイトルは `種類(範囲): 要約`（例：`fix(api): check that items and members belong to the workspace`）。種類は feat、fix、refactor、perf、test、docs、ci、chore、style など。本文は 72 桁で折り返し、なぜ変えるのかを短く書く。
- コミットに `Co-Authored-By:` などの、誰が書いたかを示す行を付けない。
- PR は main に向けて `gh pr create` で出す。本文には、変えたこと、その理由、確かめたこと（流したコマンドと結果）を書き、確かめていないことはそう書く。エージェントが作った PR は、本文の最後にその旨を 1 行書く（Claude Code なら `🤖 Generated with [Claude Code](https://claude.com/claude-code)`）。
- PR を出したら CI を待ち（`gh pr checks <番号> --watch`。実行が現れるまで数分かかることがある）、自分の変更で落ちたものを直す。main とコンフリクトしている PR では pull_request の workflow が流れないので、マージできる状態かも見る。
- PR を積み重ねたときは、土台がマージされたら上の PR の向き先を main に直す。土台のブランチにマージされたまま main に届かなかった PR がある（[#41](https://github.com/Keyhole-Koro/Synthify/pull/41)）。
- マージ、main・stage・prod への push、ブランチの削除はしない。オーナーがする。

## PR を出す前に通すもの

変えた所の CI と同じ検査を手元で通す。CI の中身は `.github/workflows/` にある。

| 変えた所 | 通すもの |
| --- | --- |
| Go（`apps/api`、`apps/worker`、`apps/eval`、`internal/`） | 変えたファイルに `gofmt -l` が何も出さない。`go vet ./...`、`go build ./...`、`go test ./...` |
| `terraform/` | `terraform fmt -check -recursive terraform`。`terraform/environments` で `terraform init -backend=false -lockfile=readonly` と `terraform validate`。plan と apply はしない |
| `apps/web` | `bun run lint`、`bun run test`、`bun run build`、変えた所の E2E（`bun run e2e:pr`。流し方は README.md） |
| 画面の見た目や動き | 上に加えて、Playwright で動きを動画に撮り、PR で見せる |
| canvas、`apps/web/src/features/landing/useLandingPageController.ts`、`apps/web/vender/paper-in-paper` | 上に加えて、Compose の環境で E2E を通してから push する。単体テストと動画では落ちず、E2E でだけ落ちた変更がある |
| `apps/monitor/ui` | `bun test src`、`bun run build` |
| `apps/tree-viewer`、`skills/synthify-tree` | `apps/tree-viewer` で `bun run typecheck`、`bun test src`、`bun run build`。skill はビルドした viewer を配るので、ビルドの結果もコミットする |
| Markdown | `node scripts/check-markdown-links.mjs` |

## デプロイと本番

- `stage` ブランチへの push で stage に、`prod` ブランチへの push で本番に、自動でデプロイされる（`.github/workflows/deploy-backend.yml`、`deploy-frontend.yml`）。main にマージしただけではデプロイされない。
- マージとデプロイ（stage・prod への push）はオーナーがする。エージェントは PR を作るところまで。
- クラウドの資源やリポジトリの設定を変えるコマンド（`terraform apply`、資源を変える `gcloud` と `firebase`、設定や secret を変える `gh`）は実行しない。手順とコマンドを示すところまで。
- paper-in-paper（`apps/web/vender/paper-in-paper`）の npm への公開は、ライブラリの `publish` ブランチへの push で走る。その push はオーナーがする。エージェントは版上げ、CHANGELOG、ライブラリの main への push と CI までを済ませ、push のコマンドを渡す。ライブラリのコミットは、Synthify の submodule をそこへ進めるコミットより先に push する。
- 障害とアラートへの対応は、手順書（`docs/runbooks/README.md`）に従う。

## 公開リポジトリと秘密

- このリポジトリは public。push したものは、PR のブランチでもすぐ誰でも読める。
- 秘密の値（鍵、トークン、パスワード、DSN、webhook の URL）はリポジトリに入れない。コード、設定、テストのデータ、ログ、コミットと PR の本文、スクリーンショットにも書かない。秘密は GitHub の secret と Secret Manager に置き、値はオーナーが入れる。
- 秘密をコミットや push してしまったら、すぐオーナーに知らせる。履歴から消しても漏れは取り消せないので、値を無効にして作り直す（手順書の「秘密が漏れたとき」）。
- セキュリティの修正は、push した時点で直す前の穴を公開することになる。push したら、すぐマージしてデプロイする（オーナー）。エージェントは、セキュリティの修正の PR を出したらすぐオーナーに伝える。

## このファイルについて

- Claude Code は、CLAUDE.md が無いときに AGENTS.md を読む。CLAUDE.md や CLAUDE.local.md を作ると（`/init` でも作られる）AGENTS.md が読まれなくなるので、作らない。どうしても要るときは、中身を `@AGENTS.md` の 1 行にする。
- ルールを変えるときは、オーナーと合意してから PR で変える。
