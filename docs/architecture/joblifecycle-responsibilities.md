# joblifecycle の責務

`packages/shared/joblifecycle` は、job まわりの**状態遷移の入口**を集約するための層。

目的は、「どの状態に遷移させるか」と「そのとき何を一緒にやるか」を service / worker / handler から剥がし、1 箇所で読めるようにすること。

## なぜ必要か

従来は次の責務が複数レイヤに分散していた。

- API service が `queued` や dispatch failure を処理する
- worker が `running` `failed` `completed` を処理する
- handler が approval request / approve / reject を repository へ直接流す
- そのたびに repository 更新と `jobstatus` 通知を個別に呼ぶ

この構造だと、同じ状態遷移が複数箇所に重複しやすい。

例えば `failed` へ遷移するときに、

- DB の status 更新
- `jobstatus` 通知
- `joblog` 記録
- 将来的には plan status / document status の更新

を毎回別々に呼ぶことになり、整合性を崩しやすい。

## 何を担当するか

`joblifecycle` は次の責務を持つ。

- 状態遷移 API を提供する
- 状態遷移時に必要な付随処理をまとめる
- 呼び出し側を「遷移の決定」ではなく「遷移 API の利用」に寄せる

今の最小単位では次を扱う。

- `NotifyQueued`
- `MarkRunning`
- `TryFail`
- `Complete`
- `RequestApproval`
- `ApproveApproval`
- `RejectApproval`

## 何を担当しないか

`joblifecycle` は repository の置き換えではない。

担当しないもの:

- SQL の実装
- DB transaction の細部
- query の最適化
- Connect handler の認可
- worker の本処理ロジック

つまり責務分解はこうなる。

- handler/service/worker:
  - 認可する
  - 入力を検証する
  - どの状態遷移 API を呼ぶか決める
- `joblifecycle`:
  - 状態遷移の入口を提供する
  - 付随する通知や更新をまとめる
- repository:
  - 実際の永続化を行う

## repository との違い

repository は「どう保存するか」を担当する。

例えば:

- `MarkProcessingJobRunning`
- `FailProcessingJob`
- `CompleteProcessingJob`
- `RequestJobApproval`
- `ApproveJobApproval`
- `RejectJobApproval`

これらは永続化 API であって、アプリケーションがどのタイミングで呼ぶかまでは表現しない。

一方 `joblifecycle` は「いま job を running にする」「いま approval を reject する」という**ユースケース単位の入口**を持つ。

## 今後ここに寄せるもの

現時点ではまだ薄い層だが、最終的には次を寄せる。

### 1. job status

- `queued`
- `running`
- `failed`
- `completed`

### 2. plan status

- `pending_approval`
- `approved`
- `rejected`
- `completed`

### 3. document lifecycle state

proto にある `DocumentLifecycleState` を、job / plan の状態から一貫して導出または更新する。

例:

- job が `queued` / `running` の間は `PROCESSING`
- approval 待ちは `PENDING_NORMALIZATION`
- job 成功で `COMPLETED`
- job 失敗で `FAILED`

### 4. 付随処理

- `jobstatus` notifier
- `joblog` 記録
- 必要なら監査ログやメトリクス

## どの順で進めるか

### Phase 1

job の `queued` / `running` / `failed` / `completed` を集約する。

### Phase 2

approval request / approve / reject を `joblifecycle` 経由にする。

この段階では、repository 内の transaction は残っていてよい。
まず handler から repository 直結をやめるのが目的。

### Phase 3

repository から「状態判断」を剥がし、`joblifecycle` 側へ寄せる。

ここで approval 系 transaction の構成も見直す。

### Phase 4

document status まで含めた完全な lifecycle 集約にする。

## 現時点の制約

今の `joblifecycle` はまだ薄い façade に近い。

理由は次の通り。

- approval 系の本体 transaction はまだ repository に残っている
- `joblog` は一部呼び出し側に残っている
- document status は未統合

ただし、この薄い段階でも次の効果がある。

- handler / service / worker から repository 直呼びを減らせる
- 「状態遷移はどこを見ればいいか」が明確になる
- 今後の集約先を先に固定できる

## 実装上の目印

現状の集約先:

- [service.go](/home/unix/Synthify/packages/shared/joblifecycle/service.go:1)

ここから呼ぶ側の代表:

- [document.go](/home/unix/Synthify/apps/api/internal/service/document.go:1)
- [worker.go](/home/unix/Synthify/apps/worker/pkg/worker/worker.go:1)
- [job.go](/home/unix/Synthify/apps/api/internal/handler/job.go:1)
