# GCS Signed Upload URL の落とし穴と固め方

ドキュメントアップロード経路を監査したときに見つけた問題と、その固め方の記録。Synthify は `CreateDocument → 署名 URL → クライアント PUT → ConfirmUpload` というよくある二段階アップロードを採用しているが、初期実装には quota 強制とサイズ照合の隙間があった。

関連:
- 署名 URL 発行: [apps/api/internal/bootstrap/bootstrap.go](../../apps/api/internal/bootstrap/bootstrap.go) の `gcsSignedDocumentUploadURLIssuer`
- 予約と確認: [apps/api/internal/repository/postgres/document.go](../../apps/api/internal/repository/postgres/document.go) の `CreateDocument` / `ConfirmDocumentUpload`
- 削除フロー: [apps/api/internal/infrastructure/storage/metadata.go](../../apps/api/internal/infrastructure/storage/metadata.go) の `*DocumentObjectStore`
- 監視: [apps/api/internal/service/document.go](../../apps/api/internal/service/document.go) の `reportUploadIncident` / `reportCreateDocumentRejection`

---

## 1. 署名 URL に `Content-Length` を含めないと quota がバイパスされる

- **症状**: `CreateDocument` で `fileSize=1` を申告して reserve させ、払い出された署名 URL に GB 級の PUT を完遂できる。Postgres 側の `storage_used + reserved + fileSize` チェックを実質ザル化できる。
- **原因**: `gcs.SignedURLOptions` の `Headers` フィールドを空のままにしていたため、V4 署名の対象に `Content-Length` が入っていなかった。GCS V4 の検証は「署名時に列挙したヘッダがリクエストに完全一致するか」だけを見るので、含めなかったヘッダは何バイトのリクエストでも通る。
- **対処**: 発行時に `opts.Headers = []string{"Content-Length:" + size}` を追加し、`IssueDocumentUploadURL(..., fileSize int64)` に `fileSize` を渡す。**ブラウザの `fetch(url, { body: file })` は `Blob` body から `Content-Length` を自動付与する**ので、クライアント側の追加実装は不要。手で `headers: { 'Content-Length': ... }` を書くと forbidden header エラーになるので注意。
- **教訓**: 「ユーザー入力 (申告サイズ) を信用しない」を実装に落とすときは、GCS 側で物理的に超過 PUT が成立しないラインまで持っていく。Postgres の事前チェックは「上限を満たすことの確認」であって「上限を超えられないことの保証」ではない。

## 2. ミスマッチ検知後に GCS object を残しっぱなしにしない

- **症状**: `ConfirmDocumentUpload` で `expected != actual` を検知すると `upload_reservations.status = 'failed'` を立てるだけで、GCS 上のオブジェクトは削除されない。reservation が 15 分で expire したケースも同じく Postgres ステータスだけが進む。
- **原因**: 失敗系の cleanup を「呼び出し元」の責務と暗黙に決めていたが、誰も呼んでいなかった。
- **対処**:
    - `DocumentObjectStore` interface に `DeleteDocumentObject` を切り、fake-gcs と本物 GCS の両方で実装。
    - `ConfirmUpload` でミスマッチを検出した瞬間に削除を呼ぶ。
    - `ExpireUploadReservations` を `RETURNING document_id, workspace_id` に変えて expire 対象を一覧で返し、各レコードについて削除を試行。
    - **削除はベストエフォート**: 404 (already gone) は成功扱い、それ以外の失敗は slog Warn + NR Custom Event に落として呼び出し元 (cron / RPC) は止めない。一時的に消えなくても次回 sweep で拾える。
- **教訓**: 二段階アップロードでは「Postgres と GCS の二箇所に状態がある」状態が普通に発生する。失敗パスでも両者の整合性を寄せる責務を、必ずどこか 1 箇所に明示的に持たせる (Synthify では `DocumentService`)。

## 3. quota 違反は slog だけでなく NR Custom Event に乗せる

- **症状**: `ErrFileTooLarge` / `ErrStorageQuotaExceeded` / `ErrUploadSizeMismatch` は repository から errors として返ってきて Connect interceptor 経由で APM の Errors にも出るが、**理由別の集計や時系列傾向は取れない**。slog でも `repository.create_document_quota_rejected` は出ているが、GCP Cloud Logging を NRQL のように使うのはつらい。
- **対処**: `DocumentService` に `nrApp` を持たせ、拒否の種類を `reason` 属性つきで Custom Event に分岐記録:
    - `UploadRejected` (reason: `file_too_large` / `storage_quota_exceeded` / `size_mismatch`) — `CreateDocument` 時の拒否
    - `UploadSizeMismatch` — `ConfirmUpload` 時の照合失敗
    - `OrphanObjectDeleteFailed` — cleanup 失敗
- **教訓**: 「ユーザー操作起点で発生する business-rule 違反」と「インフラ起点のエラー」は NR 上で別の見方をしたい。前者は CustomEvent で属性付き集計、後者は NoticeError で stack trace 集約、という棲み分けが運用上ちょうどいい。両方欲しいときだけ両方発行する (今回は `OrphanObjectDeleteFailed` がそれ)。

## 4. クライアント側の事前 size チェックは UX のためだけにある

- **観察**: billing API が `maxFileSizeBytes` を返しているのに、クライアント (`useWorkspaceTree.handleUploadWorkspaceFile`) は `file.size` を事前検査せず `CreateDocument` に投げている。
- **判断**: セキュリティ的にはサーバの事前チェック + Content-Length 強制で十分で、クライアント検査を信用してはいけない。**UX 改善 (ファイル選択直後に "大きすぎます" を出す) のためにやる**ものであって、後回しで問題はない。
- **教訓**: 「クライアントでも弾いた方が二重に安全」と書きがちだが、サーバを信用できないなら片方しかチェックする価値はない。役割を「ガード」と「UX」で分けて書くと意思決定が楽。
