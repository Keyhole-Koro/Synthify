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

