# テストマトリクス: `document_test.go`

このマトリクスは、`document_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

> **読み方の注意**: 下の「テストケース表」は *既存テストが何を確認しているか* を写したものなので、
> **テストが1件も無いメソッド／分岐はこの表に現れない**。
> 「インターフェース網羅チェック」「依存エラー軸」「未テスト分岐 (GAP)」を併読すること。
> カバレッジ数値は `go test -coverprofile` の実測 (2026-06-12)。
> `IssueImageURL` は [document_image_url](document_image_url_test.matrix.md) を参照。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

## インターフェース網羅チェック (`DocumentUsecase`)

| メソッド | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `ListDocuments` | ❌ **なし** | **0.0%** | GAP | 全分岐 — 認可拒否 / 正常 list / `ListDocuments` repo error。 |
| `GetDocument` | ❌ **なし** | **0.0%** | GAP | 全分岐 — `authorizeDocument` (0%) 経由。doc不在→Forbidden、非member拒否、正常取得。 |
| `CreateDocument` | ✅ 5件 | 77.8% | OK | `CreateDocument` repo error の非quota系、`reportCreateDocumentRejection` の各 reason。 |
| `ConfirmUpload` | ✅ 4件 | 83.3% | OK | `authorizeDocumentWrite` doc不在、二重 confirm idempotency。 |
| `StartProcessing` | ✅ 5件 | 92.3% | OK | 既存 running/queued job の早期 return、dispatch 失敗経路。 |
| `ResumeProcessing` | ◐ AutoResume 経由のみ | 66.7% | PARTIAL | **直接の専用テストが無い**。budget gate、resume 時の retry count 加算。 |

補助/job 経路の実測: `authorizeDocument` **0.0%** / `GetLatestProcessingJob` **0.0%** /
`handleDispatchFailure` **0.0%** (dispatch 失敗で job を fail にする経路) / `authorizeDocumentWrite` 50.0% /
`startProcessingJob` 74.4% / `AutoResumeFailedJobs` 70.8% / `ResumeProcessing` 66.7% /
`deleteOrphanedObject` 28.6% / `reportUploadIncident` 13.3% / `reportCreateDocumentRejection` 20.0%。

→ **`ListDocuments` / `GetDocument` の read 2経路がまるごと未テスト** (0%)。認可拒否・正常取得を移植すれば即閉じる。
**dispatch 失敗経路 (`handleDispatchFailure` 0%)** は worker 連携の重要分岐だが未確認 —
`dispatcher.GenerateExecutionPlan`/`ExecuteApprovedPlan` がエラーを返したときに job を fail にして返す挙動。

## 依存エラー軸 (dependency returns error)

document service は依存が多い。各メソッドが依存のエラーをどう扱うか。☑=テスト有 / ◐=間接 / ❌=未テスト。

| メソッド | workspaces (authz) | repo (Document) | jobs | dispatcher | objectMetadata | tree/transactor |
| --- | --- | --- | --- | --- | --- | --- |
| `ListDocuments` | ❌ | ❌ ListDocuments | - | - | - | - |
| `GetDocument` | ❌ | ❌ GetDocument | - | - | - | - |
| `CreateDocument` | ◐ Forbidden | ☑ quota系 / ❌ DB outage | - | - | - | - |
| `ConfirmUpload` | ◐ | ◐ | - | - | ☑ mismatch / ❌ lookup error | - |
| `StartProcessing` | ◐ | ◐ | ❌ GetLatestProcessingJob | ❌ dispatch 失敗 | ☑ | ❌ GetTree / WithTx |
| `ResumeProcessing` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `AutoResumeFailedJobs` | - | - | ☑ ListAllJobs (空) / ◐ | ☑ execute | - | - |

→ **dispatcher エラー / objectMetadata lookup エラー / transactor (WithTx) 失敗** の伝播が広く未テスト。
provider/repo にエラーを返させる fake を足せば `startProcessingJob` の 74% と `handleDispatchFailure` の 0% を同時に上げられる。

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestCreateDocumentRejectsOversizedFile` | `CreateDocument` | quota / validation | `MaxFileSizeBytes + 1` の PDF を作成する。 | `ErrFileTooLarge` を返し、document は nil、upload URL は空になる。 | document / job / quota reservation を作らない。 | `ErrorIs(ErrFileTooLarge)`, `Nil(doc)`, `Empty(uploadURL)` | upload 予約前のアカウント別ファイルサイズ上限。 | content-type 別の上限、実際の署名付き URL の挙動。 | content-type ごとの上限、ちょうど `MaxFileSizeBytes` の成功ケース。 | OK |
| [ ] | `TestCreateDocumentReservesQuotaUntilConfirmation` | `CreateDocument` | quota reservation | quota 200。150 bytes を予約した後、さらに 100 bytes を作成しようとする。 | 2件目の document 作成で `ErrStorageQuotaExceeded` を返す。 | 1件目の pending upload が quota を占有し続ける。 | `NoError(first)`, `ErrorIs(ErrStorageQuotaExceeded)`, `Nil(second)` | upload 確定前でも pending upload が storage quota を予約すること。 | 並行 reservation、期限切れ処理との race。 | goroutine で同時 reservation したときの超過防止。 | PARTIAL |
| [ ] | `TestCreateDocumentAllowsExactQuotaLimit` | `CreateDocument` | boundary | quota と max file size がどちらも 128。128-byte の document を作成する。 | document 作成が成功する。 | 128 bytes の reservation が作られる。 | `NoError(err)`, `NotNil(doc)` | quota ぴったりの境界値は許可されること。 | quota 0、負数または欠落した size input。 | size 0 / quota 0 の境界。 | OK |
| [ ] | `TestExpireUploadReservationsReleasesReservedQuota` | `ExpireUploadReservations`, `CreateDocument` | expiry / quota release | 150 bytes を予約し、16分後として reservation を期限切れにしてから 100-byte の replacement を作成する。 | 1件の reservation が期限切れになり、replacement は成功し、期限切れ upload は confirm できない。 | reserved quota が解放され、expired document は confirm 不可になる。 | `Equal(1, expiredCount)`, `NoError(replacement)`, `ErrorIs(ErrUploadNotConfirmed)` | 期限切れが reserved quota を解放し、遅延 confirm を防ぐこと。 | 複数の期限切れ reservation、期限切れと有効 reservation の混在。 | 複数 reservation の一括 expiry、期限直前の未 expiry ケース。 | PARTIAL |
| [ ] | `TestStartProcessingConfirmsUploadedObjectSize` | `StartProcessing` | metadata / quota commit | object metadata の size が申請した 128 bytes と一致する。 | job が作成され、workspace/requester が設定され、account storage used が 128 になる。 | upload が確定され、storage usage が加算される。 | `NotNil(job)`, `Equal(workspaceID)`, `Equal(requestedBy)`, `Equal(128, StorageUsedBytes)` | `StartProcessing` が object size を確認し、storage usage を確定すること。 | 実 object metadata provider、dispatch の副作用。 | metadata provider error、dispatcher 呼び出し検証。 | PARTIAL |
| [ ] | `TestStartProcessingRejectsSizeMismatch` | `StartProcessing` | metadata mismatch | object metadata の size は 256、申請 size は 128。 | `ErrUploadSizeMismatch` を返し、job は nil、storage used は 0 のまま。 | upload / quota commit / job 作成を行わない。 | `ErrorIs(ErrUploadSizeMismatch)`, `Nil(job)`, `Zero(StorageUsedBytes)` | size mismatch で processing を止め、quota 確定を避けること。 | 申請より小さいケース、metadata lookup error。 | metadata size が申請より小さいケース。 | PARTIAL |
| [ ] | `TestStartProcessingRejectsUnsupportedUploadedContentType` | `StartProcessing` | content-type validation | uploaded metadata の content type が `application/x-msdownload`。 | `ErrUnsupportedDocumentType` を返し、job は nil。 | processing job を作らない。 | `ErrorIs(ErrUnsupportedDocumentType)`, `Nil(job)` | processing 前に uploaded object の content type を再検証すること。 | 拡張子と content-type の曖昧な組み合わせ、実 GCS metadata の正規化。 | allowed extension + suspicious content-type の組み合わせ表。 | PARTIAL |
| [ ] | `TestConfirmUploadRejectsUnsupportedUploadedContentType` | `ConfirmUpload` | content-type validation | metadata content type が `application/x-msdownload` の upload を confirm する。 | `ErrUnsupportedDocumentType` を返し、confirmed document は nil。 | confirm / quota commit を行わない。 | `ErrorIs(ErrUnsupportedDocumentType)`, `Nil(confirmed)` | 明示的な confirm 経路で unsupported content type を拒否すること。 | 実 object metadata provider、allowlist 全体の組み合わせ。 | allowlist / denylist の table-driven test。 | PARTIAL |
| [ ] | `TestConfirmUploadAllowsOctetStreamForReadableExtension` | `ConfirmUpload` | content-type fallback | `notes.md` を `application/octet-stream` として upload して confirm する。 | confirm が成功する。 | document が confirmed になる。 | `NoError(err)`, `NotNil(confirmed)` | 読める拡張子では octet-stream upload を許容できること。 | 他の readable extension、危険な拡張子と octet-stream の組み合わせ。 | `.txt`, `.pdf`, 危険拡張子の octet-stream 比較。 | PARTIAL |
| [ ] | `TestConfirmUploadConfirmsUploadedObjectSize` | `ConfirmUpload` | metadata / quota commit | metadata size が申請 size と一致する upload を confirm する。 | confirm が成功し、account storage used が 128 になる。 | storage usage が加算される。 | `NoError(err)`, `NotNil(confirmed)`, `Equal(128, StorageUsedBytes)` | confirm 経路が size validation 後に storage usage を確定すること。 | 二重 confirm の idempotency、metadata lookup error。 | 同じ document を2回 confirm した場合の usage 重複防止。 | PARTIAL |
| [ ] | `TestConfirmUploadRejectsSizeMismatch` | `ConfirmUpload` | metadata mismatch | metadata size は 256、申請 size は 128 の upload を confirm する。 | `ErrUploadSizeMismatch` を返し、confirmed document は nil、storage used は 0 のまま。 | confirm / quota commit を行わない。 | `ErrorIs(ErrUploadSizeMismatch)`, `Nil(confirmed)`, `Zero(StorageUsedBytes)` | confirm 経路で size mismatch を拒否し、quota を確定しないこと。 | 申請より小さいケース、すでに期限切れの reservation。 | expired reservation confirm と size mismatch の優先順位。 | PARTIAL |
| [ ] | `TestStartProcessingRespectsForceReprocess` | `StartProcessing` | reprocess / idempotency | 1回 start し、job を succeeded にしてから、force なしでもう一度 start し、その後 force ありで start する。 | force なしでは既存 job を返し、force ありでは reprocess job を新規作成する。 | force ありで新しい reprocess job が増える。 | `Equal(job1.JobID, job2.JobID)`, `NotEqual(job1.JobID, job3.JobID)`, `Equal(REPROCESS_DOCUMENT)` | reprocess の重複抑止と force override。 | failed/running の既存 job、reprocess job の dispatch 挙動。 | running job があるときの force / non-force 比較。 | PARTIAL |
| [ ] | `TestAutoResumeFailedJobsRetriesLatestFailedJobOnce` | `AutoResumeFailedJobs` | retry / dispatcher | 最新の processing job が retry count 0 で failed。 | 1件 resume され、dispatcher が1回実行され、新しい latest job の retry count が 1 になる。 | 新しい retry job が作られ、dispatcher が呼ばれる。 | `Equal(1, resumed)`, `Equal(1, executeCalls)`, `NotEqual(failed.JobID, latest.JobID)`, `Equal(1, latest.RetryCount)` | auto-resume が最新の failed job を1回だけ retry すること。 | 複数 document、dispatcher error handling、古い failed job。 | dispatcher error 時の戻り値と retry job 状態。 | PARTIAL |
| [ ] | `TestAutoResumeFailedJobsSkipsAlreadyRetriedLatestFailure` | `AutoResumeFailedJobs` | retry cap | 最新の failed reprocess job の retry count が 1。 | resume されず、dispatcher も呼ばれない。 | job を追加せず、dispatcher call count は 0 のまま。 | `Equal(0, resumed)`, `Equal(0, executeCalls)` | auto-resume の上限により再 retry を防ぐこと。 | retry limit が 1 より大きい場合、複数 document で retry count が混在する場合。 | retry count の上限を設定値化した場合の境界。 | OK |
| [ ] | `TestCreateDocument_Viewer_ReturnsForbidden` | `CreateDocument` | authorization | workspace viewer が document を作成しようとする。 | `ErrForbidden` を返し、document は nil。 | document / reservation を作らない。 | `ErrorIs(ErrForbidden)`, `Nil(doc)` | viewer role は document を作成できないこと。 | anonymous user、削除済み member、share-link-only access。 | role なし user、removed member の forbidden。 | PARTIAL |
| [ ] | `TestCreateDocument_Editor_Succeeds` | `CreateDocument` | authorization | workspace editor が quota 内の PDF を作成する。 | document 作成が成功する。 | editor による document reservation が作られる。 | `NoError(err)`, `NotNil(doc)` | editor role は document を作成できること。 | editor の quota attribution 境界、owner quota と editor quota の所有関係。 | editor 作成時の quota 課金先検証。 | PARTIAL |
| [ ] | `TestStartProcessing_BudgetExceeded_ReturnsError` | `StartProcessing` | billing gate | owner account を budget exceeded にしてから processing を開始する。 | `ErrBillingBudgetExceeded` を返す。 | processing job を開始しない。 | `ErrorIs(ErrBillingBudgetExceeded)` | requester の budget gate が processing を止めること。 | billing provider integration、dispatch 中の budget 変更。 | budget exceeded 時に job が作られていないことの明示 assertion。 | PARTIAL |
| [ ] | `TestStartProcessing_EditorBudgetExceeded_BlocksEditorOnly` | `StartProcessing` | billing gate / role | editor は budget exceeded、owner は budget exceeded ではない。 | editor は拒否され、owner は同じ document の processing を開始できる。 | editor の処理だけ止まり、owner の処理は進む。 | `ErrorIs(ErrBillingBudgetExceeded)`, `NoError(owner start)` | budget check は workspace owner 全体ではなく requester に適用されること。 | owner が budget exceeded で editor が利用可能な場合、role/budget の並行変更。 | owner exceeded / editor available の逆パターン。 | PARTIAL |

## 観点別チェックグリッド

この表は、各テストケースがどの確認観点を担保しているかを横断的に見るためのものです。

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 境界値 | 認可 | quota / budget | metadata 検証 | 状態変化 | job 作成/抑止 | dispatcher | idempotency / retry | 外部依存 mock | 永続化副作用 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestCreateDocumentRejectsOversizedFile` | - | ☑ | ☑ | - | ☑ | - | ◐ | ☑ | - | - | - | ☑ |
| `TestCreateDocumentReservesQuotaUntilConfirmation` | - | ☑ | - | - | ☑ | - | ☑ | - | - | - | - | ☑ |
| `TestCreateDocumentAllowsExactQuotaLimit` | ☑ | - | ☑ | - | ☑ | - | ◐ | - | - | - | - | ☑ |
| `TestExpireUploadReservationsReleasesReservedQuota` | ☑ | ☑ | ◐ | - | ☑ | - | ☑ | - | - | - | - | ☑ |
| `TestStartProcessingConfirmsUploadedObjectSize` | ☑ | - | - | - | ☑ | ☑ | ☑ | ☑ | - | - | ☑ | ☑ |
| `TestStartProcessingRejectsSizeMismatch` | - | ☑ | - | - | ☑ | ☑ | ☑ | ☑ | - | - | ☑ | ☑ |
| `TestStartProcessingRejectsUnsupportedUploadedContentType` | - | ☑ | - | - | - | ☑ | ◐ | ☑ | - | - | ☑ | ◐ |
| `TestConfirmUploadRejectsUnsupportedUploadedContentType` | - | ☑ | - | - | - | ☑ | ☑ | - | - | - | ☑ | ◐ |
| `TestConfirmUploadAllowsOctetStreamForReadableExtension` | ☑ | - | ◐ | - | - | ☑ | ☑ | - | - | - | ☑ | ☑ |
| `TestConfirmUploadConfirmsUploadedObjectSize` | ☑ | - | - | - | ☑ | ☑ | ☑ | - | - | - | ☑ | ☑ |
| `TestConfirmUploadRejectsSizeMismatch` | - | ☑ | - | - | ☑ | ☑ | ☑ | - | - | - | ☑ | ☑ |
| `TestStartProcessingRespectsForceReprocess` | ☑ | - | - | - | - | ◐ | ☑ | ☑ | - | ☑ | ☑ | ☑ |
| `TestAutoResumeFailedJobsRetriesLatestFailedJobOnce` | ☑ | - | - | - | - | - | ☑ | ☑ | ☑ | ☑ | ☑ | ☑ |
| `TestAutoResumeFailedJobsSkipsAlreadyRetriedLatestFailure` | - | ☑ | ☑ | - | - | - | ☑ | ☑ | ☑ | ☑ | ☑ | ☑ |
| `TestCreateDocument_Viewer_ReturnsForbidden` | - | ☑ | - | ☑ | - | - | ◐ | - | - | - | - | ☑ |
| `TestCreateDocument_Editor_Succeeds` | ☑ | - | - | ☑ | ◐ | - | ◐ | - | - | - | - | ☑ |
| `TestStartProcessing_BudgetExceeded_ReturnsError` | - | ☑ | - | - | ☑ | - | ◐ | ☑ | - | - | - | ◐ |
| `TestStartProcessing_EditorBudgetExceeded_BlocksEditorOnly` | ☑ | ☑ | - | ☑ | ☑ | - | ◐ | ◐ | - | - | - | ◐ |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| 境界値 | file size 上限ぴったり、quota ぴったり、retry count 1 は確認済み。 | size 0、quota 0、metadata size が申請より小さいケース。 |
| 認可 | viewer 拒否、editor 許可は確認済み。 | role なし user、削除済み member、share-link-only access。 |
| quota / budget | upload reservation、confirm/start 時の quota commit、requester budget gate は確認済み。 | editor 作成時の quota 課金先、budget exceeded 時に job が作られないことの明示 assertion。 |
| metadata 検証 | size 一致/不一致、unsupported content type、octet-stream fallback は確認済み。 | metadata provider error、allowlist / denylist の table-driven test。 |
| dispatcher | auto-resume の execute 呼び出し有無は確認済み。 | dispatcher error 時の戻り値、retry job の状態、`StartProcessing` 経路での dispatch 副作用。 |
| idempotency / retry | force reprocess、auto-resume retry cap は確認済み。 | running / failed job が既にあるときの force / non-force 比較。 |
| 外部依存 | object metadata と dispatcher は fake で確認済み。 | 実 GCS metadata provider、実 Cloud Tasks / dispatcher integration。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 数値 / count / size

| 対象値 / 条件 | `0` | `1` | typical | `max - 1` | `max` | `max + 1` | negative | huge / overflow | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| upload file size | - | - | ☑ | - | ◐ | ☑ | - | ☑ | `TestCreateDocumentRejectsOversizedFile`, `TestCreateDocumentAllowsExactQuotaLimit` | size 0、size 1、ちょうど `MaxFileSizeBytes` を file size 上限として見る専用ケース。 |
| storage quota | - | - | ☑ | - | ☑ | ☑ | - | - | `TestCreateDocumentReservesQuotaUntilConfirmation`, `TestCreateDocumentAllowsExactQuotaLimit` | quota 0、quota 1、`quota - 1`。 |
| uploaded metadata size | - | - | ☑ | - | ☑ | ☑ | - | - | `TestStartProcessingConfirmsUploadedObjectSize`, `TestStartProcessingRejectsSizeMismatch`, `TestConfirmUploadConfirmsUploadedObjectSize`, `TestConfirmUploadRejectsSizeMismatch` | 申請より小さい size、size 0、metadata lookup error。 |
| retry count | ☑ | ☑ | - | - | ☑ | - | - | - | `TestAutoResumeFailedJobsRetriesLatestFailedJobOnce`, `TestAutoResumeFailedJobsSkipsAlreadyRetriedLatestFailure` | retry count > 1、上限が設定値化された場合の `limit - 1` / `limit + 1`。 |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| uploaded content type | - | ☑ | ☑ | ◐ | - | - | `TestStartProcessingRejectsUnsupportedUploadedContentType`, `TestConfirmUploadRejectsUnsupportedUploadedContentType`, `TestConfirmUploadAllowsOctetStreamForReadableExtension` | allowlist 全体、empty content type、拡張子と content-type の矛盾。 |
| reprocess force flag | ☑ | ☑ | - | - | - | - | `TestStartProcessingRespectsForceReprocess` | 既存 job が running / failed の場合。 |
| workspace role | - | ☑ | ☑ | ◐ | - | - | `TestCreateDocument_Viewer_ReturnsForbidden`, `TestCreateDocument_Editor_Succeeds` | owner 明示ケース、role なし user、削除済み member、share-link-only access。 |
| billing budget state | ☑ | ☑ | - | - | - | - | `TestStartProcessing_BudgetExceeded_ReturnsError`, `TestStartProcessing_EditorBudgetExceeded_BlocksEditorOnly` | owner exceeded / editor available の逆パターン、budget exceeded 時の job 未作成 assertion。 |

### 日時 / expiry

| 対象値 / 条件 | missing | past | just before | exactly at | just after | future | invalid format | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| pending upload expiry | - | - | - | - | ◐ | - | - | `TestExpireUploadReservationsReleasesReservedQuota` | expiry 直前、expiry ちょうど、複数 reservation の混在。 |
