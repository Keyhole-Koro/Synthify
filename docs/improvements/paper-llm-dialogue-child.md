# Paper 内 LLM 対話 child — 設計メモ

各 paper のヘッダ「+」から、その paper の **子として LLM 対話 paper** を生成する機能。対話 paper は
周辺 paper をコンテクストに使い（手動選択 ＋ LLM 自走選択）、回答中の paper 参照を UI 上で
クリック可能なリンクにする。設計の作業リストとして読む。

## 決定事項（2026-05-30）

- 応答配信 = **worker → Firestore 書き込み → フロントが onSnapshot 購読**（既存のジョブ進捗と同じ経路を
  再利用。server-streaming RPC を新設しない。LLM は worker のまま＝API に Vertex 権限を渡さない）
- トリガ = **フロント → API unary RPC `PostChatTurn` → worker を同期 dispatch**（turn 採番して即返し）
- ボタン = **既存「+」を対話作成に転用**（汎用 child 作成は当面割愛）
- スコープ = **フル機能**（ストリーミング対話 ＋ 手動コンテクスト選択 UI ＋ LLM 自走のコンテクスト選択）

### なぜ Firestore 経路か（当初の streaming RPC 案からの変更）

既存の進捗通知が「worker→Firestore→onSnapshot」で完成しており、対話応答もこれに乗せる方が一貫する。

- 既存資産を丸ごと再利用: 書き込み `internal/platform/job/status/notifier.go`（`workspaces/{ws}/jobs/{jobId}`
  に `MergeAll`）、購読 `apps/web/src/lib/firestore/useAuthedDoc.ts`（`onSnapshot`＋認証ガード）、
  スキーマ正本 `contracts/firestore/job-status.schema.json` → TS/Go 自動生成、worker は既に Vertex AI 認証保持。
- **初の server-streaming Connect RPC を入れずに済む**。トリガは unary で済む。
- **API に Vertex AI 権限を渡さない**（LLM は worker のまま）→ 当初プランの「API 内 Vertex クライアント」「API SA に
  aiplatform.user 付与」が不要に。
- 永続化される: 離脱→復帰しても onSnapshot が現状を読み直す（RPC streaming は切断で消える）。会話履歴も残る。

**トレードオフ（=設計で潰す点）**
1. **書き込み頻度**: トークン単位の Firestore 書き込みはコスト/レート的に不可。→ **300〜500ms or 文単位で
   部分テキストを flush** する「チャンク・ストリーミング」にする（チャット UX なら十分）。現 notifier は
   stage 単位で粗いため throttle 不要だが、対話用 notifier は throttle 必須。
2. **トリガ遅延**: 対話は初トークンを速くしたい。Cloud Tasks のエンキュー遅延を避け、対話は**同期 dispatch
   （API→worker 直叩き, `HTTPDispatcher` 系）でキック**する。
3. **landing の workspace スコープ（要決定）**: Firestore パス/ルールは `workspaces/{ws}/...` ＋要認証。
   トップの paper-in-paper が匿名/公開だと置き場所が無い。→ **ログインユーザの workspace に紐づける**か、
   匿名可の別パス＋ルールが要る。streaming RPC 案なら workspace 不要だった唯一の弱み。対話はログイン前提に
   するのが素直（デフォルト案）。

## 全体像

```
[+] (PaperHeader, 既存実装)
   → onCreateChild (LandingPageView で新規配線)
      → CREATE_CHILD_NODE  content=<DialoguePaper parentId=... />
           DialoguePaper (アプリ製 ReactNode コンテンツ):
             ├ usePaperStoreSelector で周辺 paper を収集 → 候補コンテクスト
             ├ コンテクスト選択 UI（手動チェック）＋「LLM に任せる」トグル
             │
             ├(1) API.PostChatTurn(履歴 + 候補コンテクスト本文 + pinned ids + autoSelect)
             │      API: 認証 → chatId/turnId 採番 → worker を同期 dispatch → {chatId,turnId} 即返し
             │
             └(2) useAuthedDoc で workspaces/{ws}/chats/{chatId}/turns/{turnId} を onSnapshot
                    worker: selectedContextIds → delta(throttled 部分テキスト) → final+finishReason
                    → ストリーミング描画 ＋ 回答内 <a data-paper-id> を usePaperDispatch でクリック可能化
```

会話状態の正本は Firestore の turn ドキュメント。コンテクスト本文と履歴はトリガ payload で worker に渡す
（初期実装では永続せず毎回送る。肥大化が問題になれば Postgres 永続へ）。

## 既存資産で再利用できるもの（重要）

調査で判明した「ほぼ配線だけで済む」根拠:

- ヘッダ「+」は既に `onCreateChild` を呼ぶ実装あり
  (`apps/web/vender/paper-in-paper/src/lib/react/components/PaperHeader.tsx:90-96, 153-176`)。
  ただし `LandingPageView` が `onCreateChild` を **PaperCanvas に渡していない**ため現状は非表示。
  → 転用はアプリ側の配線だけで済み、**vendored ライブラリの変更は不要**。
- `usePaperDispatch` / `usePaperStoreSelector` は公開エクスポート済み
  (`apps/web/vender/paper-in-paper/src/lib/index.ts`)。対話 paper は canvas の Provider 内で
  描画されるので、これらで `paperMap` 読み取り・`OPEN_NODE`/`FOCUS_NODE` ディスパッチが可能。
- 回答リンク: iframe コンテンツの `<a data-paper-id>` クリック→ paper を開く機構は既存
  (`apps/web/vender/paper-in-paper/src/lib/react/internal/iframeBridge.ts`)。ReactNode コンテンツでは
  同等挙動を `usePaperDispatch` で自前配線する（Phase D）。
- `Paper.content` は `ReactNode` を許容
  (`apps/web/vender/paper-in-paper/src/lib/core/types.ts:20,56`)。対話 paper はライブ React
  コンポーネントとして持たせる。
- **Firestore 進捗パターン一式**:
  - 購読フック `apps/web/src/lib/firestore/useAuthedDoc.ts`, `useAuthedCollection.ts`（`onSnapshot`＋認証ガード）
  - 書き込み `internal/platform/job/status/notifier.go`（`firestoreNotifier`, `MergeAll`）
  - スキーマ正本 → 生成: `contracts/firestore/job-status.schema.json` →
    `scripts/generate-firestore-types.mjs` → TS (`apps/web/.../firestoreJobStatus.generated.ts`) ＋
    Go (`internal/platform/job/status/firestore_job_status.generated.go`)
  - ルール `firestore.rules`（`workspaces/{ws}/jobs/{jobId}`: 認証ユーザ read 可 / client write 不可）
  - worker 側 Firestore 初期化 `apps/worker/pkg/worker/bootstrap/bootstrap.go`
- worker トリガ経路: Cloud Tasks（prod）/ 同期 HTTP dispatch（dev）。
  `apps/api/internal/infrastructure/worker/{cloudtasks_dispatcher.go,dispatcher.go}`、
  worker 受け口 `apps/worker/pkg/worker/internal_dispatch.go`（`POST /internal/dispatch-job`）。
- Connect RPC 配線（トリガ unary 用）: 透過 transport ＋ Firebase 認証インターセプタ
  (`apps/web/src/lib/connect.ts`)、生成物 `apps/web/src/gen/proto/...`、ハンドラ登録
  `apps/api/cmd/server/main.go:172-183`。認証ユーザ取得 `requireUserID(ctx)`
  (`apps/api/internal/handler/authz.go`)。
- LLM クライアント: worker の既存実装をそのまま使う
  `apps/worker/pkg/worker/llm/gemini.go`（Vertex AI / `google.golang.org/genai`,
  `gemini-3-flash-preview`）, 設定 `apps/worker/pkg/worker/config/config.go`。

---

## Phase A — Firestore chat スキーマ ＋ ルール

**目的**: 対話 turn の Firestore ドキュメント型を、既存の生成パイプラインに乗せて定義する。

**やること**:
- 新規 `contracts/firestore/chat-turn.schema.json`（job-status と同じ JSON Schema スタイル）。例フィールド:
  `chatId`, `turnId`, `workspaceId`, `paperId`(対話 paper), `status`("running"|"succeeded"|"failed"),
  `selectedContextIds`(string[]), `text`(累積回答), `finishReason`, `errorMessage`, `updatedAt`。
- `scripts/generate-firestore-types.mjs` を流用して TS/Go 型を生成。
- `firestore.rules` に `workspaces/{ws}/chats/{chatId}/turns/{turnId}`（および親 doc）を追加。
  read = 認証ユーザ、write = false（worker のみがサーバ資格で書く）。job と同方針。

## Phase B — トリガ RPC（API, unary）

新規 `contracts/connectrpc/synthify/app/v1/chat.proto`（**unary のみ。streaming 不要**）:

```proto
service ChatService {
  rpc PostChatTurn(PostChatTurnRequest) returns (PostChatTurnResponse);
}
enum ChatRole { CHAT_ROLE_UNSPECIFIED = 0; CHAT_ROLE_USER = 1; CHAT_ROLE_ASSISTANT = 2; }
message ChatMessage   { ChatRole role = 1; string text = 2; }
message ContextPaper  { string paper_id = 1; string title = 2; string description = 3; string content = 4; }
message PostChatTurnRequest {
  string workspace_id = 1;
  string chat_id = 2;                       // 既存対話の継続。空なら新規採番
  string paper_id = 3;                       // 対話 paper の id
  repeated ChatMessage history = 4;          // 末尾が最新ユーザ発話
  repeated ContextPaper candidates = 5;      // 周辺 paper の本文カタログ
  repeated string pinned_context_ids = 6;    // 手動選択（常に使用）
  bool auto_select_context = 7;              // LLM 自走選択を有効化
}
message PostChatTurnResponse { string chat_id = 1; string turn_id = 2; }
```

**やること**:
- 新規 `apps/api/internal/handler/chat.go`: `ChatHandler.PostChatTurn`。
  - `requireUserID(ctx)`、`authorizeWorkspace` で workspace 検証。
  - `chat_id`/`turn_id` を採番、turn doc を `status="running"` で初期化（API 側 notifier、または worker 起動時に worker が作る）。
  - worker を**同期 dispatch でキック**（`HTTPDispatcher` 系。Cloud Tasks のエンキュー遅延を避ける）。
    payload に history / candidates / pinned / autoSelect / chatId / turnId / paperId を載せる。
  - `{chat_id, turn_id}` を即返し（生成完了は待たない）。
- `cmd/server/main.go:172-183` 付近に
  `mux.Handle(appv1connect.NewChatServiceHandler(chatHandler, connectOptions...))` を追加。
- **API は LLM を呼ばない**（Vertex 権限不要）。

## Phase C — worker: ChatTurn 生成 ＋ Firestore 書き込み

**目的**: dispatch を受け、コンテクストを（必要なら）選び、回答を生成して turn doc に書き込む。

**やること**:
- worker の internal dispatch (`apps/worker/pkg/worker/internal_dispatch.go`) に `ChatTurn` procedure を追加。
- **コンテクスト選択 round**: `auto_select_context` の時、round-1（structured/function-call）で候補から
  関連 `paper_id` を選ばせる → `pinned_context_ids` と合算 → turn doc に `selectedContextIds` を書く。
  false なら pinned のみ。
- **回答生成**: system prompt に「参照 paper は候補 id を使い `<a data-paper-id="ID">ラベル</a>` で引用せよ」を
  明記し、選択 paper 本文 ＋ 履歴で `GenerateContentStream`。チャンクを受けつつ **throttle（300〜500ms or
  文境界）して累積 `text` を turn doc に `MergeAll` 更新**。
- 完了で `status="succeeded"` ＋ `finishReason`、失敗で `status="failed"` ＋ `errorMessage`（job notifier の
  分類に倣う）。
- 書き込みは job 用 `firestoreNotifier` と同じ Firestore client を流用し、**chat 用の薄い notifier**を追加
  （throttle ロジックを内包）。

**LLM**: 既存 `apps/worker/pkg/worker/llm` をそのまま使用（新規クライアント不要）。`GenerateContentStream` 未使用なら
genai SDK のストリーミング呼び出しを薄く足す。

## Phase D — Web: 対話 paper UI

新規 `apps/web/src/features/dialogue/`:

- `api.ts`: `createRPCClient(ChatService)` ＋ `postChatTurn()`（unary, `features/tree/api.ts` のパターン）。
- `useChatTurn.ts`: `useAuthedDoc` で `workspaces/{ws}/chats/{chatId}/turns/{turnId}` を購読
  （`useJobStatus.ts` と同形）。`{ status, text, selectedContextIds, finishReason }` を返す。
- `DialoguePaper.tsx`（`props: { parentId: PaperId }`）:
  - `usePaperStoreSelector` で `paperMap` を読み、**候補コンテクスト**収集（parent ＋ 兄弟 ＋ 祖先）。
    各候補 = `{ paperId, title, description, content(text) }`。
  - コンテクスト選択 UI: 候補チェックボックス（=`pinned_context_ids`）＋「LLM に任せる」トグル
    （=`auto_select_context`）。
  - 送信: `postChatTurn` → 返った `{chatId,turnId}` で `useChatTurn` 購読開始 → `text` を逐次描画。
    `selectedContextIds` を「使ったコンテクスト」チップ表示（クリックで開く）。
  - **回答内リンク**: `text` 中の `<a data-paper-id="ID">` を要素に変換し、クリックで `usePaperDispatch` から
    `FOCUS_NODE` ＋ `OPEN_NODE`（参照 paper の `parentId` は `paperMap` 引きで取得）。HTML サニタイズ必須。
  - メッセージ履歴はクライアント state で保持し、次ターンの `history` として送る。
- `contextText.ts`: paper.content のプレーンテキスト化ヘルパ。

**既知の制約（コンテクスト本文抽出）**: paper.content は JSX / HTML 文字列 / ContentNode[] と多様。
HTML / ContentNode はテキスト抽出できるが、**JSX paper は抽出が困難**なので `title + description`
（必要なら著者の任意 plain summary）にフォールバック。精度が要るなら optional `summary` を後追い導入。

## Phase E — 「+」を対話作成に転用（配線）

**やること**:
- `apps/web/src/features/landing/LandingPageView.tsx`（`PaperCanvas` 描画箇所 ~122-133）に
  **`onCreateChild` を追加**:
  ```tsx
  onCreateChild={(parentId, create) =>
    create({ title: 'AI 対話', description: '周辺コンテクストで質問', hue: /* accent */,
             content: <DialoguePaper parentId={parentId} /> })}
  ```
- 生成 paper を保持するため `onPaperMapChange` も渡し、`useLandingPageController.ts` の paperMap を
  `useMemo` 固定値から **state 保持 ＋ マージ**に変更。

## Phase F — 設定 / インフラ / 要決定

- **API は Vertex 権限不要**（当初プランから削除）。worker は既に Vertex AI 認証保持。
- Firestore ルールデプロイ（Phase A の chat パス）。
- **要決定**: 対話を**ログイン必須**にする（デフォルト案、workspace スコープに素直に乗る）か、匿名 landing でも
  使えるよう匿名 workspace/別パス＋ルールを用意するか。匿名対応は Firestore のスコープ設計が増えるため、
  初期はログイン必須を推奨。

---

## 変更ファイル一覧（代表）

- 追加: `contracts/firestore/chat-turn.schema.json`（→ TS/Go 生成）
- 追加: `contracts/connectrpc/synthify/app/v1/chat.proto`（unary PostChatTurn）
- 変更: `firestore.rules`（chat パス追加）
- 追加: `apps/api/internal/handler/chat.go`（トリガのみ。LLM 呼ばない）
- 変更: `apps/api/cmd/server/main.go`（ハンドラ登録）
- 変更/追加: `apps/worker/pkg/worker/internal_dispatch.go`（ChatTurn procedure）＋ chat 用 throttle notifier
- 追加: `apps/web/src/features/dialogue/{api.ts,useChatTurn.ts,DialoguePaper.tsx,contextText.ts}`
- 変更: `apps/web/src/features/landing/LandingPageView.tsx`, `useLandingPageController.ts`
- 生成: `internal/gen/...`, `apps/web/src/gen/proto/...`, firestore 型生成物
- **不要**: `apps/web/vender/paper-in-paper/` 変更 / API への Vertex 権限・LLM クライアント

## Phase 間の依存関係

```
Phase A (firestore schema/rules) ─┐
Phase B (trigger RPC) ────────────┤
                                  ├─▶ Phase C (worker ChatTurn → Firestore)
                                  │        └─▶ Phase D (Web 対話 UI: post + onSnapshot)
                                  │                 └─▶ Phase E (「+」配線)
Phase F (rules deploy / 認証方針) は C/D のデプロイ前提として並行
```

**最短経路**: A/B → C（worker が turn doc に書けること確認）→ D（最小チャット）→ コンテクスト選択 →
リンク化 → E。段階コミット推奨。

## 検証

1. 生成: `buf generate`（proto）＋ `scripts/generate-firestore-types.mjs`（firestore 型）。Go/TS 型を確認。
2. API: `go build ./...` ＋ 既存テスト。`PostChatTurn` を叩くと turn doc が `running` で作られ ids が返る。
3. worker: dispatch を受け、turn doc の `text` が throttle 更新 → `succeeded`＋`finishReason` になることを確認
   （Firestore エミュレータ可）。
4. Web: 型チェック ＋ `dev` 起動。手動 E2E:
   - 任意 paper の「+」→ 子に対話 paper が開く。
   - 質問送信 → onSnapshot 経由で回答が逐次描画される（チャンク単位）。
   - 回答内 paper リンクをクリック → 参照 paper が開く（`FOCUS_NODE`/`OPEN_NODE`）。
   - コンテクストのチェック切替で送信内容が変わる。「LLM に任せる」ON で `selectedContextIds` チップが出る。
   - 途中でリロードしても onSnapshot が turn の現状を復元する。

## 関連の既存ドキュメント

- Firestore 進捗パターン: `internal/platform/job/status/notifier.go`,
  `apps/web/src/lib/firestore/useAuthedDoc.ts`, `contracts/firestore/job-status.schema.json`
- paper-in-paper レイアウト挙動: [paper-in-paper-sibling-share.md](paper-in-paper-sibling-share.md),
  [paper-in-paper-importance-direction.md](paper-in-paper-importance-direction.md)
- LLM Worker 設計: [../llm-worker-architecture.md](../llm-worker-architecture.md)
- routing 統合構想（/w/[id] 廃止しトップ paper-in-paper に統合）はメモリ参照
