# フロントエンド エラーハンドリング改善

## 目的

フロントエンドで発生するエラーを、Next.js のデフォルト画面に落とさず、ユーザーが状況を理解して復旧できる形にする。

現在の `This page couldn’t load` は最終防衛ラインとしては必要だが、通常の API 失敗、ログイン失敗、アップロード失敗、Firestore 購読失敗、部分的な tree 読み込み失敗までページ全体エラーに見せるべきではない。

## 現状の課題

- `apps/web/app/error.tsx` / `global-error.tsx` がなく、予期しない render/runtime error が Next.js の素の画面になる。
- 実際に使われている Connect RPC transport (`src/lib/connect.ts`) で共通エラー正規化がない。
- `src/lib/rpc.ts` には `ApiError` があるが、現状の generated Connect client 経路では使われていない。
- Firebase Auth / Connect RPC / fetch upload / Firestore listener のエラー形式が UI までばらばらに流れている。
- 一部の操作失敗が `console.error` だけで UI に出ない。
  - Google ログイン失敗
  - workspace 作成失敗
  - subtree lazy load 失敗
- workspace 一覧エラーは表示されるが、再試行がなく logout しかできない。
- billing 系 paper は個別に `error` state を持っているが、表示・再試行・文言が不統一。
- `useWorkspaceTree` の `throw new Error("workspace not found")` は状態ズレでページ全体エラーに落ちる可能性がある。
- Firestore listener error が `err.message` のまま UI に渡る。
- upload error が `Upload failed: status` だけで、サイズ上限、署名 URL 期限切れ、CORS、ネットワーク失敗を区別できない。

## エラー分類

### FatalError

対象:

- render 中の予期しない例外
- アプリ初期化失敗
- 回復不能な client runtime error

扱い:

- `app/error.tsx` と `app/global-error.tsx` で受ける。
- ユーザーには stack trace を見せない。
- `再読み込み` と `ホームに戻る` を出す。
- 開発・運用向けには `error.digest` や error name/message をログ送信する。

### AuthError

対象:

- `auth/unauthorized-domain`
- popup blocked
- popup closed/cancelled
- token refresh failed
- Firebase Auth emulator/network failure

扱い:

- ログイン paper 内に表示する。
- ユーザー操作の近くに出す。
- 管理者向けの詳細コードは console/reporting に残す。

### ApiError

対象:

- Connect RPC の `Unauthenticated`, `PermissionDenied`, `NotFound`, `InvalidArgument`, `Unavailable`, `Internal`
- API への network failure
- timeout

扱い:

- Connect interceptor で共通の `AppError` に正規化する。
- UI では status/code ではなく用途別メッセージに変換する。
- 401/403 はログイン状態や権限の問題として扱う。
- 5xx/Unavailable は再試行可能な一時エラーとして扱う。

### PanelLoadError

対象:

- workspace 一覧取得失敗
- billing 情報取得失敗
- usage / invoice / payment method 取得失敗
- job log / job status 一覧取得失敗

扱い:

- page 全体ではなく該当 paper/panel 内に表示する。
- `再試行` を出す。
- 既存データがある場合は stale data を残し、小さな warning として表示する。

### ActionError

対象:

- workspace 作成
- workspace rename
- document upload
- checkout / portal URL 取得
- budget 保存
- logout

扱い:

- 操作したボタンやフォームの近くに表示する。
- 操作中 state と失敗 state を分ける。
- 再実行できるものはその場で再試行できるようにする。

### BackgroundSyncError

対象:

- Firestore job status listener
- Firestore workspace job list listener
- subtree lazy load
- localStorage persistence

扱い:

- ページ全体を壊さない。
- 小さな warning として表示する。
- ユーザー操作が必要な場合だけ明示的な再試行を出す。

## 実装計画

### 1. App-level boundary を追加する

追加:

- `apps/web/app/error.tsx`
- `apps/web/app/global-error.tsx`
- 必要なら `apps/web/app/loading.tsx`

表示:

- `ページを読み込めませんでした`
- `一時的な問題が発生しました。再読み込みしてください。`
- `再読み込み`
- `ホームに戻る`

この画面は最後の砦であり、通常の API/操作失敗をここに流さない。

### 2. 共通エラー型を作る

追加候補:

- `apps/web/src/lib/errors.ts`
- `apps/web/src/lib/errorMessages.ts`

型のイメージ:

```ts
type AppErrorKind =
  | 'auth'
  | 'permission'
  | 'not_found'
  | 'validation'
  | 'network'
  | 'unavailable'
  | 'server'
  | 'unknown';

type AppError = {
  kind: AppErrorKind;
  message: string;
  retryable: boolean;
  cause?: unknown;
  code?: string;
  status?: number;
};
```

変換対象:

- Firebase Auth error
- Connect error
- fetch/network error
- Firestore listener error
- upload response error

### 3. Connect transport で正規化する

`apps/web/src/lib/connect.ts` の interceptor で:

- `getAuthHeaders()` の失敗を `AppError` に変換する。
- `next(req)` の Connect error を `AppError` に変換する。
- raw error をそのまま feature に投げない。

`src/lib/rpc.ts` は未使用なら削除するか、generated Connect client 側と同じ error utility を使う形に整理する。

### 4. 共通 UI を作る

追加候補:

- `InlineError`
- `PanelError`
- `PanelLoading`
- `EmptyState`
- `RetryButton`

用途:

- paper/panel 内の読み込み失敗
- 操作失敗
- background sync warning

文言と余白、色、再試行ボタンの見た目を統一する。

### 5. AuthPaper を改善する

対象:

- `useAuthState`
- `AuthPaper`
- `SocialLogin`

変更:

- `authError` state を持つ。
- `signInWithGoogleSession()` の失敗を UI に表示する。
- `auth/unauthorized-domain` は prod では一般文言、開発時は詳細文言にする。
- popup cancelled は error として強く出さない。

### 6. Workspace list を改善する

対象:

- `useAuthState`
- `WorkspaceListContent`
- `WorkspaceError`
- `CreateWorkspaceForm`

変更:

- workspace 一覧 load error と create error を分ける。
- `WorkspaceError` に `再試行` を追加する。
- workspace 作成失敗をボタン付近に表示する。
- 既存 workspace がある状態で refresh に失敗した場合は、一覧を消さず warning にする。

### 7. Workspace tree を改善する

対象:

- `useWorkspaceTree`
- `WorkspacePaper`
- `WorkspaceJobList`

変更:

- `buildWsPaper` で `throw new Error("workspace not found")` しない。
- workspace 欠落時は paper 内に `ワークスペースが見つかりません` を表示する。
- `refreshWorkspaceTree` 失敗を呼び出し元に返し、paper 内に表示できるようにする。
- `loadSubtreeForItem` 失敗を UI に反映し、対象 node/paper に `再試行` を出す。

### 8. Upload を改善する

対象:

- `documents/api.ts`
- `WorkspacePaper`
- `RootUploadPaper`

変更:

- upload error を分類する。
  - file too large
  - signed URL expired
  - GCS/CORS
  - network
  - unknown
- ユーザー向け文言を分類ごとに出す。
- retry 可能な失敗は同じ file で再試行できるようにする。

### 9. Billing papers を統一する

対象:

- `BillingPanel`
- `BillingSummary`
- `CurrentPlanPaper`
- `UsagePaper`
- `InvoicePaper`
- `UpgradePaper`
- `ManagePaper`
- `BudgetSettingsPaper`

変更:

- 共通 `PanelLoading` / `PanelError` を使う。
- 読み込み失敗には `再試行` を出す。
- checkout / portal / budget save は `ActionError` として操作付近に出す。
- stale data がある場合は消さずに warning を出す。

### 10. Firestore listener を改善する

対象:

- `useJobStatus`
- `useWorkspaceJobStatuses`

変更:

- Firestore error を `AppError` に変換する。
- permission denied / unauthenticated / unavailable を分類する。
- UI には生 `err.message` を出さない。
- listener が復旧したら warning を消す。

## 優先度

### P0

- `app/error.tsx` / `global-error.tsx`
- Connect/Firebase error 正規化
- AuthPaper の操作エラー表示
- Workspace list の再試行
- `useWorkspaceTree` の render-time throw 排除

### P1

- Billing papers の共通 `PanelError` 化
- Upload error 分類
- Firestore listener error 分類
- workspace 作成失敗 UI

### P2

- subtree node 単位の retry
- stale data warning
- error reporting service 連携
- toast/notification system

## 受け入れ条件

- 通常の API 失敗で `This page couldn’t load` が出ない。
- ログイン失敗はログイン paper 内に表示される。
- workspace 一覧失敗は paper 内で再試行できる。
- workspace 作成、rename、upload、billing 操作の失敗が操作箇所に表示される。
- Firestore listener 失敗で画面全体が壊れない。
- 予期しない render error は Synthify の error page に落ちる。
- ユーザー向け UI に raw stack trace や Firebase/Connect の生 message を出さない。
