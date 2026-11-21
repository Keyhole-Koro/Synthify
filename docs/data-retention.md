# データの保持と削除

利用者のデータ（上げた文書、そこから作ったツリー、アカウント）をどこに持ち、いつ・どう消すかをまとめる。実装の正本は `apps/api/internal/application/document_delete.go` と `apps/api/internal/repository/postgres/data_deletion.go`。

## 保存先

| 場所 | 中身 |
| --- | --- |
| CockroachDB | 文書（`documents`・`document_files`）、本文の断片（`document_chunks`）、ノードの題・説明・本文（`tree_items`）とその出典（`item_sources`）、処理ジョブ（`document_processing_jobs`）とそのログ・変更記録（`job_logs`・`job_mutation_logs`。変更前後のノードの本文を含む）、アップロードの予約、利用者・アカウント・課金 |
| GCS（uploads バケット） | `{ws}/{doc}/source/`（原本）、`{ws}/{doc}/extracted/`（worker の展開・コピー）、`.cache/v1/{doc}/`（worker の grep キャッシュ。本文の抜粋）、`.checkpoints/{job}/`（worker の段階ごとの出力） |
| Firestore | `workspaces/{ws}/jobs/{job}`（ジョブの進み具合の表示用） |

GCS の場所の組み立ては `internal/platform/storage/prefixes.go` にまとめ、worker が書く場所がこの中に収まることを `apps/worker/pkg/worker/storage/deletion_layout_test.go` で確かめる。

## 文書の削除

ワークスペースの「ドキュメント」一覧の「削除」から消す（`DocumentService.DeleteDocument`）。確認を 1 回挟む。

- 消せる人: ワークスペースの所有者はどの文書でも、編集者は自分が上げた文書だけ。閲覧者は消せない。
- アップロード中（予約が有効な 15 分）と処理中（キュー待ち・実行中・再実行待ち）は断る（`FailedPrecondition`）。消した後にアップロードや worker が行やファイルを書き戻すため。30 分以上進んでいないジョブは止まったものとみなし、削除を妨げない。
- 消すものと順番（その場で消し、利用者からは元に戻せない）:
  1. GCS: その文書の原本・展開・キャッシュと、その文書の全ジョブのチェックポイント。バケットは版を残す設定なので、旧版も世代を指定して消す。
  2. DB（1 トランザクション）: その文書だけから作られたノード、文書の行と、外部キーでつながる行（ファイル、チャンク、予約、ジョブとそのログ・変更記録・チェックポイント、出典）。確定済みのアップロードの容量をアカウントに戻す。
- GCS の削除に失敗したら DB には触れず、文書は一覧に残る。もう一度削除すれば続きから消える。逆の順にすると、どの行からも指されないファイルが残る。

### ノードの扱い

ツリーはワークスペースに 1 本で、文書は出典にすぎない（[tree-node-workspace-ownership.md](improvements/tree-node-workspace-ownership.md)）。ノードが「その文書から作られた」とは、その文書を出典（`item_sources`）に持つか、その文書の処理ジョブがそのノードを作った・統合した・書き換えた（`job_mutation_logs`）こと。モデルが出典のチャンクを挙げなかったノードには出典の行が無いので、ジョブの記録も見る。

- その文書だけから作られたノードは消す。残る子は、消えない一番近い祖先の下に移す（無ければワークスペース直下）。ワークスペースの表紙のノードを作った文書を消すと、残ったノードがワークスペース直下に並び、画面は先頭のノードを表紙にする。
- 別の文書からも作られたノードは残し、消す文書の出典だけを外す。作った文書が 1 つになれば `cross_document` を外す。
- 手で作ったノード（出典もジョブの記録も無い）は残す。
- 人が手を入れて守っているノード（`governance_state` が `human_curated` か `locked`）は、その文書だけから作られていても消さない。worker による書き換えを断る（`UpdateItemSummaryHTMLWithCapability` の "item is protected"）のと同じで、自動の変更から守る。消す文書の出典の行は外れ、そのあとは手で作ったノードと同じに扱われる。子もそのまま残る。

### 消えずに残るもの

- 人が手を入れて守っているノード（上の `human_curated`・`locked`）の本文。消した文書から作られた内容を含みうる。守りを外す手段も、ノードを消す手段も、今は画面と API に無いので、消すには運営者の作業が要る。
- 別の文書と統合されたノードの本文。統合のときにモデルが書き直した本文には、消した文書の内容が混ざりうる。どこまでがどの文書の内容かは分けられない。
- 別の文書のジョブの変更記録にある、統合する前のノードの本文。
- Firestore のジョブの表示。終わったジョブは 7 日後に TTL が消す（`terraform/services/platform/main.tf`）。終わらずに止まったジョブの分は残る。
- 課金の記録（`usage_events` はアカウントにつながり、ジョブの ID を文字列で持つだけ）。
- Cloud Logging と New Relic に出たファイル名と ID。保持期間はそれぞれの設定に従う。
- GCS のソフト削除。Cloud Storage は、消したオブジェクトをバケットのソフト削除の期間（Terraform では設定しておらず、既定なら 7 日）残し、その間は運営者が戻せる。実際のバケットの設定は未確認。
- DB のバックアップ。CockroachDB Serverless のバックアップの保持期間は未確認。

## 権限

API のサービスアカウントは uploads バケットに `roles/storage.objectUser` を持つ（`terraform/services/platform/main.tf` の `api_uploads_deleter`）。`objectCreator` と `objectViewer` だけでは消せない。
