# Chat Turn Contract

paper 内 LLM 対話の契約仕様。トリガ RPC (`ChatService.PostChatTurn`) と、応答配信に使う
Firestore turn ドキュメントの 2 つを正本として定義する。

実装計画は [../improvements/paper-llm-dialogue-child.md](../improvements/paper-llm-dialogue-child.md)、
上位構想（クリック位置 anchor + intent 判定）は
[../improvements/contextual-paper-llm-collaboration.md](../improvements/contextual-paper-llm-collaboration.md)。
本ドキュメントは **形と責任分界だけ**を決め、UI 挙動と phase 分割は扱わない。

## 全体の流れ

```
web  --PostChatTurn (unary)-->  API  --同期 dispatch-->  worker
                                 |                        |
                          {chatId, turnId}          Vertex AI structured output
                                 |                        |
                                 v                        v
web  <--onSnapshot--------  users/{uid}/chats/{chatId}/turns/{turnId}
```

責任分界:

- **API は LLM を呼ばない**。認証・認可・ID 採番・dispatch のみ。Vertex AI 権限を API に渡さない。
- **worker だけが turn ドキュメントを書く**。client write は rules で禁止。
- **Firestore は正本ではない**。job status と同じ扱いで、転送中の状態と UI 通知のためだけに使う。
  会話履歴の永続化については「履歴の持ち方」を参照。
- **workspace 認可は API が担い、Firestore rules は担わない**。turn doc のパスを
  `users/{uid}/...` にすることでこれを構造的に保証する。詳細は「Firestore rules」を参照。

## 1. トリガ RPC

新規 `contracts/connectrpc/synthify/app/v1/chat.proto`。**unary のみ**。server-streaming は導入しない。

```proto
syntax = "proto3";

package synthify.app.v1;

option go_package = "github.com/synthify/backend/internal/gen/synthify/app/v1;appv1";

service ChatService {
  rpc PostChatTurn(PostChatTurnRequest) returns (PostChatTurnResponse);
}

enum ChatRole {
  CHAT_ROLE_UNSPECIFIED = 0;
  CHAT_ROLE_USER        = 1;
  CHAT_ROLE_ASSISTANT   = 2;
}

message ChatMessage {
  ChatRole role = 1;
  string   text = 2;
}

// 周辺 paper の本文カタログ。web が paperMap から収集して送る。
message ContextPaper {
  string paper_id    = 1;
  string title       = 2;
  string description = 3;
  string content     = 4;  // プレーンテキスト化済み
}

message PostChatTurnRequest {
  string                workspace_id       = 1;
  string                chat_id            = 2;  // 空なら新規採番
  string                paper_id           = 3;  // 対話 paper の id
  repeated ChatMessage  history            = 4;  // 末尾が最新ユーザ発話
  repeated ContextPaper candidates         = 5;
  repeated string       pinned_context_ids = 6;  // 手動選択。常に使用
  bool                  auto_select_context = 7; // LLM 自走選択
}

message PostChatTurnResponse {
  string chat_id = 1;
  string turn_id = 2;
}
```

### API 側の契約

- `requireUserID(ctx)` で認証、`authorizeWorkspace` で workspace 所有を検証する。
- `chat_id` が空なら採番、非空なら **その chat が当該 workspace に属することを検証**する。
  他人の chat_id を渡して turn を注入できてはいけない。
- `turn_id` を採番し、turn doc を `status="running"` で初期化する。初期化を API 側で行うのは、
  worker 到達前にフロントが購読を開始しても doc が存在するようにするため。
- worker を**同期 dispatch** (`HTTPDispatcher`) でキックする。Cloud Tasks のエンキュー遅延は
  対話の初トークン遅延として体感されるため、chat では使わない。
- `{chat_id, turn_id}` を即返す。生成完了は待たない。

### リクエストサイズの上限

`candidates` に paper 本文を丸ごと載せるため、リクエストは容易に肥大化する。契約として上限を決める:

| 項目 | 上限 | 超過時 |
|---|---:|---|
| `candidates` 件数 | 32 | `InvalidArgument` |
| `candidates[].content` | 8 KiB / 件 | API 側で切り詰め |
| `candidates` 合計 | 128 KiB | `InvalidArgument` |
| `history` 件数 | 40 | 古い順に落とす |
| `history[].text` | 8 KiB / 件 | API 側で切り詰め |

切り詰めは API 側で行い、worker には正規化済みの payload だけを渡す。Connect のデフォルト
メッセージ上限にぶつかる前に、こちらの契約で弾く。

## 2. Firestore turn ドキュメント

### Path

`users/{uid}/chats/{chatId}/turns/{turnId}`

親 doc `users/{uid}/chats/{chatId}` にはメタデータ（`workspaceId`, `paperId`, `createdAt`,
`updatedAt`, `title`）を置く。turn の一覧購読に使う。

### なぜ workspace 配下ではなくユーザー配下か

job status は `workspaces/{ws}/jobs/{jobId}` だが、chat は **意図的に別の置き方**をする。

パスに workspace を含めると、rules は「このユーザーはこの workspace を読めるか」に答える
必要が生じる。その答えは Postgres の `IsWorkspaceAccessible`
([db/queries/workspaces.sql:70-80](../../db/queries/workspaces.sql#L70-L80)) にしかなく、
中身は `account_users` 経由 **OR** `workspace_members` 経由の OR 判定 + `deleted_at IS NULL` である。
Firestore には account / member のデータが一切存在しないため、rules で照合するには
**Postgres から Firestore へメンバーシップをミラーする機構が新規に要る**。しかもその
ミラーは SQL 側の意味論（account 経由の owner 相当扱い、論理削除、role 優先順位）を
別言語で再実装し、変更のたびに追従し続けなければならない。追従漏れはそのまま認可の穴になる。

パスを `users/{uid}/...` にすると、rules の条件は `request.auth.uid == uid` だけになり、
**照合先データが不要**になる。workspace 認可は `PostChatTurn` の `authorizeWorkspace`
([apps/api/internal/handler/authz.go:49](../../apps/api/internal/handler/authz.go#L49)) で
既に行われているため、認可が緩むわけではない。認可判断を API 側の 1 箇所に閉じ込め、
Firestore rules を意味論を持たない層に保つ、というのがこの契約の立場である。

`workspaceId` はパスから外れるがドキュメントのフィールドとして保持するので、情報は失われない。

### 帰結: 対話は個人に属する

この設計では、共有 workspace であっても **他人の対話 turn は見えない**。viewer が owner の
質問を読むことはできない。これは制約ではなく仕様判断であり、「試しに聞いてみた質問」は
個人のものとして扱う。

対話の結果を共有したい場合は、**tree item として明示的に保存する**（`create_child` / Apply）。
その成果物は Postgres 上の tree item なので、既存の workspace 認可がそのまま適用され、
workspace メンバー全員が見られる。「試した質問は自分だけ、残したい回答は paper として全員に」
という切り分けであり、[contextual-paper-llm-collaboration.md](../improvements/contextual-paper-llm-collaboration.md)
の preview → Apply モデルと一致する。

対話の往復そのものを tree item にはしない。tree は抽象から具体へ辿る知識構造であり、
思考の途中経過が混ざると paper map の見渡しやすさが損なわれるため。

### Schema

正本は `contracts/firestore/chat-turn.schema.json`（新規）。

| Field | Type | Required | Writer | Notes |
|---|---|---:|---|---|
| `chatId` | string | yes | api/worker | Chat ID。 |
| `turnId` | string | yes | api/worker | Turn ID。 |
| `workspaceId` | string | yes | api/worker | 親 workspace ID。 |
| `paperId` | string | yes | api | 対話 paper の id。 |
| `status` | string | yes | api/worker | `running`, `succeeded`, `failed`。 |
| `content` | string | yes | worker | 回答本体。`ContentNode[]` の JSON 文字列。**差分ではなく毎回全体**を上書きする。 |
| `commands` | string | no | worker | 自動実行してよいキャンバス操作。`Command[]` の JSON 文字列。 |
| `proposals` | string | no | worker | Apply 待ちの構造変更。`Command[]` の JSON 文字列。 |
| `selectedContextIds` | string[] | no | worker | 実際に使った context paper id。 |
| `finishReason` | string | no | worker | `stop`, `max_tokens`, `safety`, `recitation`, `other`。 |
| `reason` | string | no | worker | 失敗分類。job status の enum に揃える。 |
| `errorMessage` | string | yes | worker | 失敗時以外は空。 |
| `promptTokens` | integer | no | worker | 課金・計測用。 |
| `completionTokens` | integer | no | worker | 同上。 |
| `createdAt` | string | no | api | RFC3339。 |
| `updatedAt` | string | yes | api/worker | RFC3339。 |
| `completedAt` | string | no | worker | RFC3339。 |
| `expiresAt` | Timestamp | no | worker | TTL 用。native Firestore Timestamp。 |

`status` は job status の 4 値から `queued` を落とした 3 値にする。chat は同期 dispatch で
キューイング段階を持たないため、`queued` が観測されることはない。

### content は文字列ではなく ContentNode[]

回答本体は Markdown / HTML 文字列ではなく、paper-in-paper の `ContentNode[]`
([core/types.ts:5-19](../../apps/web/vender/paper-in-paper/src/lib/core/types.ts#L5-L19)) とする。
この型は `(LLM-friendly, JSON-serializable)` と型定義に明記されており、`paper-link` / `card` は
レンダラ側でクリック・キーボード操作まで配線済み
([PaperContentNodes.tsx:49-95](../../apps/web/vender/paper-in-paper/src/lib/react/components/PaperContentNodes.tsx#L49-L95))。

したがって **LLM に HTML を生成させてサニタイズ・パースする経路は採らない**。structured output で
`ContentNode[]` を直接生成させれば、paper 参照が最初からクリック可能な UI 要素になる。
`PaperContent` は `ContentNode[]` を受理するので、そのまま paper の content にできる。

構想の全体像は
[../improvements/paper-native-llm-interaction.md](../improvements/paper-native-llm-interaction.md)。

### JSON 文字列として持つ理由

`content` / `commands` / `proposals` は **Firestore 上は不透明な JSON 文字列**として持ち、
web 側で parse して `ContentNode[]` / `Command[]` にキャストする。

`ContentNode` は再帰的な discriminated union であり、`scripts/generate-firestore-types.mjs` の
現行実装（enum / array / プリミティブのみ対応）では**型を生成できない**。`$ref` + `oneOf` の
スキーマ表現と generator の再帰型対応を入れれば構造化フィールドにできるが、それは
section 6 の一般化に加えて必要な追加作業になる。

初期実装では文字列で持ち、schema による検証を web 側の parse で代替する。構造化が必要に
なった時点で移行する（Firestore の値は JSON なので、移行時に読み替えは効く）。

**型の正本は paper-in-paper の TypeScript 定義**であり、この契約はそれを参照する。
worker 側は同じ形の JSON を生成する責任を負う。

### 差分ではなく全体を上書き

`content` に差分を書いて client 側で連結する設計は**採らない**。理由:

- `onSnapshot` は全ての中間状態の配信を保証しない。書き込みが速いと複数更新が合体して届き、
  差分方式では欠落する。全体上書きなら最新スナップショットだけで常に正しい。
- リロード・再購読時に、履歴を遡らずその時点の全体が得られる。

代償として 1 write のペイロードが応答長に比例して増える。Firestore の 1 MiB / document 上限に
対し、応答は後述の `maxOutputTokens` で抑える。

### commands と proposals の分離

LLM が発行する `Command` は、**自動実行してよいものと Apply が要るものを worker 側で分離**して
別フィールドに書く。web は `commands` を無条件に dispatch してよく、`proposals` は preview として
表示し、ユーザーの Apply を待つ。

| フィールド | 含めてよい Command |
|---|---|
| `commands` | `OPEN_NODE`, `CLOSE_NODE`, `FOCUS_NODE`, `PIN_NODE`, `UNPIN_NODE` |
| `proposals` | `CREATE_CHILD_NODE`, `PATCH_NODE`, `MOVE_NODE`, `MERGE_PAPERS`, `UPSERT_PAPERS` |

`commands` に入れてよいのは、**元に戻せる**か**状態を失わない**視界操作に限る。

`DELETE_NODE` および `__SYNC_*` 系は **LLM の tool schema から除外**し、どちらのフィールドにも
現れてはならない。削除が妥当と判断される場合も、LLM は削除を提案せずユーザーに判断を委ねる。

worker は分類を**信頼せず検証する**。`commands` に構造変更系が混入していた場合は
`proposals` に移すか破棄する。web 側も dispatch 前に同じ検証を行う（多層防御）。

## 3. 生成と配信の粒度

### トークン単位のストリーミングは採らない

`content` を `ContentNode[]` にした以上、structured output は JSON 全体が揃うまで parse できず、
部分的な JSON は不正なため、**チャンクごとに `ContentNode[]` を組み立てられない**。これは
配信路の性質ではなく生成方式の帰結であり、ローカル経路（配信が自由な場合）でも変わらない。

### 生成単位は section

**1 回の generate で 1 section を返させ、section ごとに `content` 配列へ追記する。**
これを両経路の共通形とする。

- `content` は常に**その時点までの配列全体**を上書きする（差分追記にしない。理由は
  「差分ではなく全体を上書き」と同じ）。
- 生成単位は section。トークン単位にはしない。
- 完了時に `status="succeeded"` と `finishReason` を確定させる。

粒度は粗い（1 section ＝ 数秒）が、生成完了まで何も出ない状態は避けられる。

### 配信粒度は経路に委ねる

**契約が約束するのは「`content` は常に `ContentNode[]` の全体である」ことだけで、
それが何回に分けて届くかは経路の自由とする。**

| | サーバー経路 | ローカル経路 |
|---|---|---|
| 配信 | Firestore `onSnapshot` | SSE / WebSocket 等 |
| 書き込み/送信の回数 | section 数 + 1 | 経路の裁量（制約なし） |
| コスト | Firestore write 課金 | ゼロ |

サーバー経路は Firestore の sustained write 上限（およそ 1 write/sec）があるため、
**section より細かい単位で書いてはならない**。ローカル経路にはこの制約がないので、
より細かく流してもよい。

いずれの場合も **web 側の描画コードは共通**になる（「最新の `content` 全体を描く」だけ）。
差分連結を採らなかった判断がここで効く。

### 書き込みの流れ（サーバー経路）

| 段階 | 書き込み |
|---|---|
| turn 開始 | `status="running"`（API が初期化） |
| section ごと | `content`（その時点までの全体）を上書き |
| 生成完了 | `content` + `commands` + `status="succeeded"` |
| 失敗 | `status="failed"` + `errorMessage` |

throttle 機構は不要。section 単位なら sustained write 上限に達しない。

`GenerateTextStream`（section 7）の実装は**不要**。section 単位の生成は
`GenerateStructured` の複数回呼び出しで実現できる。

## 4. Firestore rules

現行 [firestore.rules](../../firestore.rules) は `workspaces/{ws}/jobs/{jobId}` のみ許可し、
残りは `match /{document=**}` で全 deny。chat パスを追加しないと購読が permission-denied になる。

chat は job とは**独立した match ブロック**を持ち、条件も異なる。job の
`request.auth != null` を chat に流用しない。

```
match /users/{uid}/chats/{chatId} {
  allow get, list: if request.auth != null && request.auth.uid == uid;
  allow write: if false;

  match /turns/{turnId} {
    allow get, list: if request.auth != null && request.auth.uid == uid;
    allow write: if false;
  }
}
```

`request.auth.uid == uid` はパス上の所有者と認証済みユーザーの一致だけを見る。
外部データの参照 (`exists()` / `get()`) を含まないため、ミラーも reconcile も不要で、
rules の評価コストも一定である。

### job status 側の穴は別問題として残る

現行 rules の job status は `request.auth != null` だけで、**workspace の所有者チェックを
していない**。認証さえ通れば他人の workspace の job status が読める。

これは chat とは独立した既存の問題であり、本契約では解消しない。chat が
`users/{uid}/...` に分離されたことで job の条件を引き継がずに済むため、両者を切り離して
扱える。job status の是正は [workspace-sharing.md](../improvements/workspace-sharing.md) の
メンバーモデル整理と合わせて別途行う。

## 5. TTL

turn doc も job status と同じく `expiresAt` で自動削除する。ただし **TTL policy は
collection group 単位**で、terraform の `google_firestore_field` は
`collection = "jobs"` / `field = "expiresAt"` を pin している
([terraform/services/platform/main.tf:285](../../terraform/services/platform/main.tf#L285))。

したがって `turns` collection group に対して**別の `google_firestore_field` リソースが要る**。
既存のものを流用することはできない。

```hcl
resource "google_firestore_field" "chat_turns_expires_at_ttl" {
  project    = var.project_id
  database   = "(default)"
  collection = "turns"
  field      = "expiresAt"

  ttl_config {}
  index_config {}
}
```

保持期間は job status の 7 日より長くする。job の進捗は終われば用済みだが、対話履歴は
ユーザーが後から読み返す。**30 日**を初期値とし、`running` 中は `expiresAt` を書かない
（job notifier と同方針）。

親 doc `chats/{chatId}` は turns とは別 collection group なので、TTL を効かせるなら
さらにもう 1 リソースが要る。初期実装では親 doc に TTL を設定せず、孤児として残ることを
受容する（doc あたりのサイズが小さいため）。

## 6. 型生成

`scripts/generate-firestore-types.mjs` は **job-status 専用にハードコードされている**。
schema path・出力先 2 つ・Go package 名 `jobstatus`・型名 prefix `FirestoreJobStatus`・
`goNameOverrides` がすべてトップレベル定数として直書きされている。

chat-turn schema を通すには、**generator を複数スキーマ対応に一般化する必要がある**。
これは契約作業に含まれる前提作業であり、schema を書いただけでは型が出ない。

一般化の形（推奨）:

```js
const targets = [
  {
    schema: 'contracts/firestore/job-status.schema.json',
    tsOut:  'apps/web/src/features/jobs/firestore/firestoreJobStatus.generated.ts',
    goOut:  'internal/platform/job/status/firestore_job_status.generated.go',
    goPackage: 'jobstatus',
    typeName:  'FirestoreJobStatus',
    goNameOverrides: { /* 既存 */ },
  },
  {
    schema: 'contracts/firestore/chat-turn.schema.json',
    tsOut:  'apps/web/src/features/dialogue/firestore/firestoreChatTurn.generated.ts',
    goOut:  'internal/platform/chat/status/firestore_chat_turn.generated.go',
    goPackage: 'chatstatus',
    typeName:  'FirestoreChatTurn',
    goNameOverrides: { chatId: 'ChatID', turnId: 'TurnID', workspaceId: 'WorkspaceID',
                       paperId: 'PaperID', selectedContextIds: 'SelectedContextIDs' },
  },
];
```

既存の生成物が 1 バイトも変わらないことを確認してから chat 分を足す。`status` enum の
Go 型名が `${typeName}State` に特別扱いされている点も引き継ぐ。

### 再帰型対応は不要

`content` / `commands` / `proposals` を JSON 文字列として持つ（section 2）ことで、
generator への**再帰型対応は不要**になる。これらのフィールドは schema 上ただの `string` で、
`ContentNode` の構造は generator の関知するところではない。

生成される TS 型は `content: string` になるため、web 側で parse とキャストを行うヘルパを
別途持つ。型の正本は paper-in-paper 側にあるので、そこから import する:

```ts
import type { ContentNode, Command } from '@/vender/paper-in-paper';

function parseContent(raw: string): ContentNode[] { /* parse + 検証 */ }
```

将来 `content` を構造化フィールドに移す場合は、`$ref` + `oneOf` のスキーマ表現と
generator の再帰型対応が必要になる。その時点で判断する。

## 7. LLM 呼び出し

既存の `GenerateStructured`
([llm/gemini.go:45](../../apps/worker/pkg/worker/llm/gemini.go#L45)) を使う。
**新規のストリーミング実装は不要**。

`ContentNode[]` を structured output で生成させるため、response schema には
paper-in-paper の型定義に対応する JSON Schema を渡す。型の正本は
[core/types.ts](../../apps/web/vender/paper-in-paper/src/lib/core/types.ts) の TypeScript 定義。

section 単位の生成（section 3）は、**`GenerateStructured` を section ごとに呼ぶ**ことで実現する。
新しい LLM API は要らない。

### 契約上の注意

- **retry が section 単位でそのまま使える**。[llm/retry.go](../../apps/worker/pkg/worker/llm/retry.go)
  と [llm/ratelimiter.go](../../apps/worker/pkg/worker/llm/ratelimiter.go) を通常通り適用できる。
  1 回の呼び出しが 1 section を完結させるので、**失敗した section だけをやり直せる**。
  トークン単位のストリーミングにつきまとう「部分出力が二重になる」問題が発生しない —
  これが本方式の実務上の利点である。
- **usage は呼び出しごとに確定する**。turn 全体の usage は section 分の合計になる。
  metering は section ごとに加算する。
- `maxOutputTokens` を明示する。Firestore の 1 MiB / document 上限と、全体上書き方式の
  write サイズを抑えるため。**section あたり** 4096 tokens を初期値とする。
- **section 数の上限を決める**。モデルが section を無限に増やして turn が終わらない事態を
  防ぐ。初期値 8 sections。上限に達したら打ち切り、`finishReason` に反映する。
- **生成された JSON の検証は worker の責任**。`ContentNode` の判別子が未知の値だったり、
  `paper-link` / `card` の `paperId` が候補に無い id を指していた場合は、その node を落とすか
  `text` node に降格する。web に不正な構造を渡さない。検証は **section ごとに**行い、
  不正な section は落として次に進む。
- 同様に `commands` / `proposals` の分類も worker が検証する（section 2 参照）。
  これらは **turn 完了時に 1 回だけ**確定させる（section ごとに視界操作を発行しない）。

### トークン単位が必要になったら

本当に必要になった時点で初めて検討する。その場合もサーバー経路は Firestore の write 上限が
効くため、**ローカル経路限定の最適化**になる可能性が高い。

## 8. 履歴の持ち方

初期実装では会話履歴を **Postgres に永続しない**。web が client state で保持し、毎ターン
`history` として送り直す。Firestore の turn doc は転送中の状態であり、TTL で消える。

これは意図的な割り切りで、以下を受容する:

- リロードすると会話履歴が消える（turn doc は残るが、web は履歴として読み直さない）。
- 毎ターンのリクエストが履歴の長さに比例して増える。上限は「リクエストサイズの上限」で規定。

肥大化またはリロード耐性が問題になった時点で Postgres 永続へ移す。その際 Firestore 側の
契約は変えずに済む（Firestore は依然として転送中の状態のみ）。

## 未決定事項

- **匿名ユーザー**: パスが `users/{uid}/...` なので Firebase Auth の uid が必須。匿名認証
  (`signInAnonymously`) を使えば匿名 landing でも uid は得られるが、`PostChatTurn` 側が
  workspace を要求するため結局 workspace が要る。初期はログイン必須とする。
- **並行 turn**: 同一 chat で前の turn が `running` のまま次を投げた場合の扱い。
  初期案は API 側で拒否（`FailedPrecondition`）。
- **キャンセル**: 生成中の turn を止める RPC を持つか。初期実装では持たず、client が購読を
  やめるだけ（worker は最後まで生成し課金される）。

## Test Plan

契約レベルで満たすべきこと:

- `PostChatTurn` が認証なしで `Unauthenticated`、他人の workspace で `PermissionDenied` を返す。
- 他人の `chat_id` を渡した turn 注入が `PermissionDenied` になる。
- `candidates` が件数・サイズ上限を超えると `InvalidArgument`。
- turn doc が `status="running"` で初期化され、`{chat_id, turn_id}` が即返る。
- 書き込みが section 単位であり、section より細かい単位で書かれない。
- 1 turn の書き込み回数が section 数 + 1 に収まる。
- section 数が上限（初期値 8）を超えると打ち切られ、`finishReason` に反映される。
- 1 つの section の生成が失敗しても、その section だけが retry される。
- `content` が parse 可能な `ContentNode[]` であり、常に全体が書かれている。
- 生成完了で `status="succeeded"` + `finishReason`、失敗で `status="failed"` + `errorMessage`。
- 未知の判別子を持つ node が落とされるか `text` node に降格される。
- 候補に無い `paperId` を指す `paper-link` / `card` が web に渡らない。
- `commands` に構造変更系 Command が混入した場合、worker が `proposals` に移すか破棄する。
- **`DELETE_NODE` と `__SYNC_*` がどちらのフィールドにも現れない**。
- web が `commands` を dispatch する前に同じ検証を行う（多層防御）。
- rules により client からの turn doc write が拒否される。
- 生成した TS/Go 型が schema と一致し、**job-status 側の生成物が変化していない**。
- 終端 turn に `expiresAt` が入り、`running` 中は入らない。
