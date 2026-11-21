# データの保持と削除

利用者のデータを、どこに置き、どこへ送り、いつ消すかをまとめる。プライバシーポリシーの本文（[`apps/web/src/features/legal/privacyPolicy.ts`](../../apps/web/src/features/legal/privacyPolicy.ts)）の「取得する情報」「取扱いの委託」「保存期間と削除」「Cookie 等の利用及び外部への送信」は、この文書を元に書いている。保存先、送り先、保存期間を変えたら、両方を直す。

MediShare の `docs/data-retention.md` に当たる文書。MediShare と違い、Synthify には定期の掃除ジョブも、退会も、文書の削除も無い。今あるものと無いものを分けて書く。

## 保存先

| データ | 保存先 | 地域 | 根拠 |
| --- | --- | --- | --- |
| ログインの記録（UID、メール、表示名、プロフィール画像の URL、ログイン日時） | Firebase Authentication | Google の全域 | `apps/web/src/features/auth/session.ts`（`signInWithPopup(GoogleAuthProvider)`） |
| 利用者（UID、メール、表示名、登録日時、最終ログイン日時） | CockroachDB の `users` | ❓ | `db/migrations/0012_users.up.sql`、`apps/api/internal/application/user.go` |
| ワークスペース、メンバー、共有リンク | CockroachDB の `workspaces`・`workspace_members`・`workspace_share_links` | ❓ | `0002`・`0015`・`0019`・`0020` |
| 文書のメタ情報（ファイル名、形式、大きさ、アップロードした人） | CockroachDB の `documents`・`document_files`・`upload_reservations` | ❓ | `0003` |
| 文書の原本と、worker が展開したファイル | GCS の uploads バケット（`{ws}/{doc}/source/`・`{ws}/{doc}/extracted/`） | `asia-northeast1` | `terraform/services/platform/main.tf`、`internal/platform/storage/gcsurls.go` |
| 文書から抽出したテキストの断片と埋め込み | CockroachDB の `document_chunks` | ❓ | `0003` |
| 知識ツリー | CockroachDB の `tree_items`・`item_sources`・`item_source_refs`・`item_aliases` | ❓ | `0008`・`0018`・`0021` |
| ジョブの記録（状態、計画、承認、変更の前後の値、ログ） | CockroachDB の `document_processing_jobs`・`job_*` | ❓ | `0007` |
| LLM が作ったツール（コードと動作確認の入力） | CockroachDB の `dynamic_tools` | ❓ | `0011` |
| ジョブの進み具合（画面への通知） | Firestore の `workspaces/{ws}/jobs/{job}` | ❓ | `contracts/firestore/job-status.schema.json` |
| 課金（プラン、Stripe の顧客 ID、利用量、予算、クレジット、請求書、カードのブランド・下 4 桁・有効期限、Webhook の記録） | CockroachDB の `accounts`・`usage_events`・`account_usage_daily`・`account_credits`・`invoices`・`payment_methods`・`billing_events` | ❓ | `0001`・`0004`・`0006`・`0009`・`0010` |
| カード番号 | Stripe だけ（Checkout の画面で入力し、サーバーを通らない） | Stripe | `apps/api/internal/infrastructure/stripe/provider.go` |
| サーバーのログ（API はメソッド・パス・状態だけ。Cloud Run のリクエストログに接続元の IP と UA） | Cloud Logging | ❓ | `internal/platform/httpmiddleware/logger.go` |
| 監視（APM、ブラウザー） | New Relic | US | `terraform/environments/main.tf`（`region = "US"`） |

❓ は Terraform の外で決まっていて、リポジトリからは分からない。プライバシーポリシーでは【要確認】にしている。

## ブラウザーに置くもの

本サービスは Cookie を発行しない。

| キー | 中身 | 消えるとき |
| --- | --- | --- |
| IndexedDB `firebaseLocalStorageDb` | Firebase の ID トークンと更新トークン、プロフィール | ログアウト |
| localStorage `synthify:has_session` | ログインしているかの印 | ログアウト |
| localStorage `synthify_expansion_map:<UID>`・`synthify_focused_item_id:<UID>` | キャンバスで開いていた項目 | ログアウト（`usePersistentPaperOpenState.ts`） |
| localStorage `synthify_workspace_snapshot:<UID>` | ワークスペースの一覧の写し（ID、名前、容量） | 消さない（`clearWorkspaceSnapshot` を呼ぶ所が無い） |
| localStorage `NRBA_SESSION` | New Relic Browser のセッション | New Relic の期限 |

## 外部に送るもの

| 送り先 | 送るもの | 根拠 |
| --- | --- | --- |
| Gemini（`synthify-gemini-api-key` が空でなければ Gemini API、空なら Vertex AI の `global`） | 文書の全文、抽出したテキストと画像、埋め込みを作るテキスト | `apps/worker/pkg/worker/llm/factory.go`、`apps/worker/pkg/worker/config/config.go` |
| New Relic APM | API と worker のトレースとエラー、カスタムイベント（`UploadRejected` に user_id・workspace_id・ファイル名） | `apps/api/internal/application/document.go` |
| New Relic Browser | ページの URL（`token=` などは伏せる）、エラー、`console.error`・`warn` の全部、Firebase の UID（`setUserId`） | `apps/web/src/lib/newrelic/browser.ts` |
| Stripe | 顧客名 `account-<UID>`、`account_id`、トークン数のメーターイベント | `apps/api/internal/infrastructure/stripe/provider.go`、`apps/api/internal/repository/postgres/workspace.go` |
| Discord | アラートの題名と New Relic の URL だけ。利用者のデータは送らない | `terraform/services/monitoring/notification.tf` |
| Google Fonts | 知識ツリーの HTML を開くと、ブラウザーがフォントを読み込む（IP と UA が Google に渡る） | `apps/web/vender/paper-in-paper/src/lib/react/internal/iframeBridge.ts` |

stg は `log_llm_payload = "true"` で、LLM に渡した文書の中身が Cloud Logging に出る（`terraform/tfvars/stage.tfvars`）。prod は既定の `false`。

## 消える仕組み（今あるもの）

- Firestore のジョブ: 完了か失敗のときに `expiresAt` を 7 日後にし（`internal/platform/job/status/notifier.go` の `jobStatusTTL`）、TTL で消える（`terraform/services/platform/main.tf` の `jobs_expires_at_ttl`）。動いているジョブは期限を持たない。
- メンバーの削除: `workspace_members` の行を消す（`db/queries/workspaces.sql` の `RemoveWorkspaceMember`）。
- 共有リンクの取り消し: `revoked_at` を立てるだけで、行とトークンは残る（`RevokeShareLink`）。
- ワークスペースの削除: `deleted_at` を立てるだけ（`SoftDeleteWorkspace`）。一覧とアクセスの判定から外れるが、文書、ツリー、ジョブの記録、GCS のファイルは残る。
- 確定しなかったアップロード: 予約は 15 分で切れる。API の 5 分ごとの掃除（`apps/api/internal/bootstrap/application.go` の `expireUploadReservations`）が予約を `expired` にし、GCS のファイルを消す。`documents` と `upload_reservations` の行は残り、消したファイルはバージョニングで以前の版として残る。

## 消えないもの（今無いもの）

- 文書を消す RPC（`DocumentService` に削除が無い）。
- 退会（アカウントの削除）。Firebase Authentication の記録も残る。
- 論理削除したワークスペースを後で DB と GCS から消す処理。
- GCS の以前の版の片付け。バケットはバージョニングが有効で（`terraform/modules/gcs_bucket/main.tf`、既定 `true`）、lifecycle の規則が無いので、上書きや削除で以前の版になったファイルが残り続ける。
- 保存期間の上限。どのデータも、消すまで残る。

これらを作るまで、プライバシーポリシーは「消去を希望する場合は窓口に連絡、運営者が消す」と書いている。下の手順で、頼まれたら手で消す。

## 頼まれて消すとき（手作業）

まだ通しで試していない。最初に頼まれたときに stg で一度通し、この節を直す。

1. 本人であることを確かめる（登録しているメールアドレスから連絡が来ているか）。
2. `SELECT user_id FROM users WHERE email = $1` で UID を、`SELECT account_id FROM account_users WHERE user_id = $1` で account を引く。
3. その account のワークスペースの ID を控える（`SELECT workspace_id FROM workspaces WHERE account_id = $1`、論理削除したものも含む）。
4. GCS からワークスペースごとに、以前の版も含めて消す（`{ws}/` の下を `--all-versions` で）。
5. `DELETE FROM accounts WHERE account_id = $1`。外部キーの `ON DELETE CASCADE` で、`account_users`、利用量、クレジット、請求書、カード、ワークスペースと、その下の文書、チャンク、ツリー、ジョブ、メンバー、共有リンク、LLM が作ったツールまで消える。`billing_events` は外部キーが無く残る（決済の記録として残すかは決める）。
6. 他人のワークスペースのメンバーとしての行（`DELETE FROM workspace_members WHERE user_id = $1`）と、`users` の行を消す。
7. Firebase Authentication から利用者を消す。Firestore のジョブは 7 日で消えるが、急ぐなら `workspaces/{ws}/jobs` を消す。
8. Stripe の顧客は、請求の記録を残す期間を確かめてから扱う。Cloud Logging と New Relic のデータは、それぞれの保存期間で消える。
9. DB のバックアップに残る期間（下）が過ぎるまでは復元できることを、本人に伝える。

## バックアップと復元できる期間

| 対象 | 期間 | 設定 |
| --- | --- | --- |
| CockroachDB | ❓ | CockroachDB Cloud の管理画面（Terraform の外） |
| GCS | 以前の版を消すまで無期限 | `terraform/modules/gcs_bucket/main.tf`（lifecycle の規則なし） |
| Firestore | バックアップなし（ジョブの一時的な状態だけ） | — |

## 確かめていないこと

プライバシーポリシーの【要確認】と同じもの。

- CockroachDB のクラスタの地域とバックアップの保持期間
- Firestore のロケーション
- Cloud Logging（`_Default` バケット）と New Relic の保存期間
- 本番の Gemini が、入力を Google の製品の改善に使わない条件か（Gemini API の有料枠か Vertex AI か）

## 足すとき

- PR #51 のワークスペースのチャットは、会話を DB に残す。マージしたら、この文書の「保存先」と、プライバシーポリシーの「取得する情報」「保存期間と削除」に会話を足し、保存期間を決める。
