ちょっと特殊な実装にしているところがいくつかあるので、ここにまとめておきます。

### API と worker の分担

OLTP 系の処理は API でやっています。具体的には、認証、認可、課金、workspace/document/job の作成などです。ドキュメント処理、chunking、LLM 呼び出し、tree 生成などの時間がかかるものは worker に渡します。

worker は Cloud Run の内部向けサービスとして動かしていて、API から Connect RPC で呼びます。worker から API に戻す内部呼び出しは service token で認証します。ブラウザから直接叩ける経路とは分けています。

### Firestore による完了通知

worker は job の進捗と完了状態を Firestore に書きます。frontend は Firestore の `onSnapshot()` で job status を購読します。完了を検知したら、それをトリガーに成果物を API に問い合わせます。

Firestore は通知と表示用の状態で、workspace/document/tree の正本は Postgres/API 側にあります。この構成により、frontend から API への progress polling をなくしています。

### ブラウザから GCS へ直接アップロード

ドキュメントの実体は API を経由しません。API は `CreateDocument` で document record、upload reservation、quota check、signed upload URL 発行をまとめてやります。frontend は返ってきた signed URL に対して、GCS へ直接アップロードします。

アップロード後に `StartProcessing` を呼びます。API は GCS object metadata を確認してから worker に処理を渡します。予約なしで upload URL だけを発行する経路は使いません。

### Worker は GCS FUSE でドキュメントを読む

worker は GCS bucket を GCS FUSE で filesystem として mount します。これにより、LLM がドキュメントを扱いやすくなります。例えば `grep` などのコマンドを tool として使いやすいです。

LLM による検索で、OpenSearch や PostgreSQL の部分一致検索を主経路にはしていません。worker が filesystem 上の成果物と tool を使って探索します。派生成果物や cache も filesystem 前提で扱えるので、LLM が段階的に読み進めやすいです。

### ログと観測データ

ログは用途ごとに保存先を分けています。

Cloud Run / Cloud Logging には、API と worker が stdout に出す `slog` JSON、Cloud Run request log、system log が残ります。主に障害時の一次切り分け用で、`job_id`、`workspace_id`、`document_id`、trace id などで調査します。

New Relic は API / worker の処理時間、Connect RPC、DB 呼び出し、error、明示的に送った custom event を集約します。stack trace、遅い処理、job 失敗の傾向分析に使います。

frontend には New Relic Browser を入れています。ブラウザ内で起きた JavaScript error、error boundary で捕捉した error、画面表示・遷移の遅さ、API 通信時間を New Relic に送ります。token、cookie、document 本文、アップロード内容、個人情報の本文値は送らず、カスタム属性は `user_id` など調査に必要な ID に限定します。

worker の job 実行ログは、Cloud Logging とは別に DB の `job_logs` / `job_mutation_logs` にも残します。これは job 単位の進行表示、失敗理由の確認、監査用です。
