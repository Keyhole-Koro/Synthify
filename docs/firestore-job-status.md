# Firestore Job Status

このドキュメントは、document processing の進捗通知に Firestore を使う方針を定義する。

## 目的

worker の進捗と完了をフロントへ低遅延で伝える。

ただし、知識グラフや document の正本は Postgres に残し、Firestore は通知用の realtime projection に限定する。

## なぜ Firestore を使うか

今回ほしいのは次の 2 つである。

- `queued` / `running` / `completed` / `failed` をすぐ UI に反映したい
- stage 名もリアルタイムに見せたい

これを API の polling や SSE だけで先に作ると、次のコストがある。

- API 側に配信経路を持つ必要がある
- `LISTEN/NOTIFY` や SSE の運用を先に固める必要がある
- worker の完了通知だけのために API の責務が重くなる

Firestore を通知専用に使えば、worker が status を更新し、フロントが subscribe するだけで進捗表示が成立する。

## 役割分担

### Postgres に残すもの

- `documents`
- `document_processing_jobs`
- `graphs`
- `nodes`
- `edges`
- `document_chunks`
- ACL / membership / account 系

Postgres が source of truth である。

### Firestore に置くもの

- document processing job の現在状態
- 現在の stage
- エラーメッセージ
- 完了通知のための最小メタデータ

Firestore は source of truth ではなく、realtime read model である。

## データフロー

```txt
Browser
  ├─ API で document 作成 / 処理開始
  ├─ Firestore の job status を subscribe
  └─ completed を受けたら API で document / graph を再取得

API
  ├─ document_processing_jobs を Postgres に作成
  ├─ Firestore に queued を書く
  └─ worker に dispatch

Worker
  ├─ Postgres に job 状態・成果物を書き込む
  └─ Firestore に running / stage / failed / completed を書く
```

## Cloud Run 構成

この方針では、当面の Cloud Run は次の 2 つで足りる。

- `api`
- `worker`

### `api` の責務

- 認証・認可
- document / graph の CRUD
- `document_processing_jobs` の作成
- Firestore への `queued` 反映
- worker への dispatch
- 完了後にフロントが再取得するための read API

### `worker` の責務

- pipeline 実行
- Postgres への成果物保存
- Firestore への `running` / `currentStage` / `failed` / `completed` 反映

### 追加の Cloud Run を作らない理由

Firestore を通知経路として採用するなら、`DB 通知専用 service` や `realtime gateway` を先に作る必要はない。

理由:

- フロントは Firestore を直接 subscribe できる
- worker 完了通知のためだけに常駐 service を増やす必要がない
- API は結果取得 API に集中できる
- `LISTEN/NOTIFY` や SSE の運用を後回しにできる

つまり今の段階では、`api` と `worker` の 2 service 構成が最も単純である。

## 追加 Cloud Run が必要になる条件

次の要件が出たら、Cloud Run を 1 つ増やす余地がある。

- Firestore をやめて API 経由の SSE / WebSocket に統一したい
- Postgres `LISTEN/NOTIFY` を専用常駐プロセスでさばきたい
- worker 完了以外にも多数のイベントを配信したい
- Pub/Sub を受けて複数クライアント向けに fan-out したい

その場合の候補は次のいずれかである。

- `realtime` service
  SSE / WebSocket 配信専用
- `event-router` service
  Pub/Sub / DB change を受けて Firestore や API 向けに再配信

ただし、今はそこまでの複雑さを持ち込まない。

## Firestore のドキュメント形

コレクション:

```txt
workspaces/{workspaceId}/jobs/{jobId}
```

ドキュメント例:

```json
{
  "jobId": "job_01...",
  "jobType": "process_document",
  "documentId": "doc_01...",
  "workspaceId": "ws_01...",
  "graphId": "gr_01...",
  "status": "running",
  "currentStage": "pass1_extraction",
  "errorMessage": "",
  "createdAt": "2026-04-16T12:34:56Z",
  "updatedAt": "2026-04-16T12:35:12Z"
}
```

## status の意味

- `queued`
  API が job を作成し、worker 実行待ち
- `running`
  worker が処理中
- `completed`
  worker が完了し、正本データの保存まで済んでいる
- `failed`
  worker が失敗し、再取得しても新しい成果物はない

`completed` は「Firestore 上で完了」ではなく、「Postgres への保存を含めて処理完了」を意味する。

## stage の扱い

`currentStage` には pipeline の stage 名をそのまま入れる。

例:

- `raw_intake`
- `normalization`
- `text_extraction`
- `semantic_chunking`
- `brief_generation`
- `pass1_extraction`
- `pass2_synthesis`
- `persistence`
- `html_summary_generation`

UI はこの文字列をそのまま見せてもよいし、表示用ラベルへ変換してもよい。

## フロントの責務

フロントは Firestore を購読して job の変化を反映する。

- `queued` / `running`
  バッジと stage を更新する
- `failed`
  エラーメッセージを出す
- `completed`
  API で `ListDocuments` や graph API を再取得する

重要なのは、完了時に最終データを Firestore から読まないこと。

Firestore は通知専用であり、最終的な graph / node / edge は API 経由で Postgres から取り直す。

## 採用しないもの

今回は次を採用しない。

- Firestore を graph の正本にする
- nodes / edges / chunks を Firestore に複製する
- worker 完了通知のために先に Pub/Sub を入れる
- 先に `LISTEN/NOTIFY + SSE` を本実装する

理由は、今ほしいのが「進捗通知」であって「配信基盤の一般化」ではないため。

## 注意点

- Firestore に ACL の正本を置かない
- Firestore の job status を業務データの正本として扱わない
- `completed` 後の UI 更新は API 再取得で整合を取る
- 古い job document の cleanup は別途考える

## 将来の移行余地

将来的に次へ移行する余地は残す。

- API の SSE
- Postgres `LISTEN/NOTIFY`
- Pub/Sub によるイベント配信

その場合でも、当面は Firestore job status を UI 通知用に残してよい。

ただし、複数の通知経路が同時に増えると責務がぶれるので、移行時はどれを正規経路にするかを明示すること。
