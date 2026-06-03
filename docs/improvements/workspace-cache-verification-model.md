# Workspace cache / verification model

## 背景

リロード直後や deploy 直後に、フロントが `TreeService/GetTree` を呼び、`permission_denied` / `forbidden` が表示されることがある。

バックエンドの workspace 認可は単純で、`TreeService.GetTree` は現在の Firebase uid が `account_users` 経由で対象 workspace の account に紐づいている場合だけ許可する。

したがって、以下の状態で tree API を叩くと 403 になり得る。

- Firebase auth は復元中だが、server-side user/account provisioning がまだ完了していない
- `signInUser()` が `account_users` を作る前に `listWorkspaces()` / `GetTree` が走る
- localStorage に残った open state が、現在のユーザーで未確認の workspace を開こうとする
- deploy 後の cold start / DB 接続初期化で、認証・provisioning・UI 復元の順序が揺れる

一方で、UX としてはリロード直後に前回の workspace 表示を即復元したい。そこで、**表示用 cache** と **API 呼び出し許可の根拠** を分離する。

## 基本方針

採用するモデルは **cache-first render, authz-gated refresh**。

1. 前回の UI 状態は cache から即復元してよい
2. cache は権限の根拠にしない
3. tree/document/workspace の API 呼び出しは verified workspace だけに許可する
4. verified workspace は `signInUser()` 完了後の `listWorkspaces()` に含まれる workspace とする
5. cached workspace が verified されなかった場合は、ユーザー向け 403 ではなく reconciliation の結果として扱う

## 状態モデル

workspace ごとに概念上は以下の状態を持つ。

```ts
type WorkspaceVerificationState =
  | { status: 'cached' }
  | { status: 'verifying' }
  | { status: 'verified' }
  | { status: 'rejected'; reason: 'not_listed' | 'permission_denied' | 'deleted' };
```

現行実装では簡略化して、`verifiedWorkspaceIds: Set<string>` を持つ。

- `workspaces`: UI 表示用。cache 由来と API 由来を含む
- `verifiedWorkspaceIds`: tree API を叩いてよい workspace id

`workspaces` に存在することと、`verifiedWorkspaceIds` に存在することは別の意味を持つ。

## 起動時のフロー

### 1. Auth 復元

Firebase の `onAuthStateChanged` で user が復元される。

user が無い場合:

- workspace list を空にする
- verified workspace を空にする
- anonymous state として扱う

user がある場合:

- verified workspace を空にする
- user scope の cached workspace snapshot を読む
- cached workspace を `workspaces` に入れて即表示する
- `signInUser()` を実行する

### 2. Server provisioning

`signInUser()` は server 側で以下を保証する。

- `users` row の upsert
- `accounts` row の取得/作成
- `account_users` row の取得/作成

`TreeService` / `WorkspaceService` の認可は `account_users` に依存するため、`listWorkspaces()` は `signInUser()` の後に実行する。

### 3. Authoritative workspace list

`listWorkspaces()` の結果を authoritative source として扱う。

成功した場合:

- `workspaces` を API 結果に置き換える
- `verifiedWorkspaceIds` を API 結果の workspace id set にする
- workspace snapshot cache を更新する

失敗した場合:

- cached `workspaces` は残す
- `verifiedWorkspaceIds` は空にする
- API refresh は retry まで止める

これにより、前回表示は残るが、未確認 workspace に対する `GetTree` / `GetSubtree` は発火しない。

## API gate

tree 系 API 呼び出しの前に必ず gate を通す。

```ts
function canFetchWorkspaceTree(workspaceId: string): boolean {
  return verifiedWorkspaceIds.has(workspaceId);
}
```

`useWorkspaceTree` では以下を gate 対象にする。

- `refreshWorkspaceTree`
- `loadSubtree`
- `mergeDocumentRootIntoTree`
- job completed による document root merge

未 verified workspace では、workspace paper shell の表示と open state 更新までは許可する。ただし backend API は呼ばない。

### 新規作成 workspace

`createWorkspace()` / root upload で作られた workspace は API 成功時点で現在ユーザーに属することが確定している。

そのため、作成直後に `markWorkspaceVerified(workspaceId)` してよい。

## Reconciliation

cached workspace と authoritative workspace list は次の規則で合流する。

```ts
const verifiedIds = new Set(serverWorkspaces.map((w) => w.workspaceId));

for (const cached of cachedWorkspaces) {
  if (verifiedIds.has(cached.workspaceId)) {
    markVerified(cached.workspaceId);
    continue;
  }

  markRejected(cached.workspaceId, 'not_listed');
}
```

ただし UI では、`not_listed` を即座に破壊的削除として扱わない。

理由:

- 一時的な API 障害で cache を失うのを避ける
- deploy / cold start 中の不安定なタイミングを吸収する
- ユーザーには「権限がない」と断定するより、静かに閉じる方が自然な場合が多い

推奨 UX:

- `listWorkspaces()` 成功後に list に無い workspace は open state から閉じる
- cache eviction は安定して `not_listed` が確認された後、または明示 logout / delete 時に行う
- `PermissionDenied` は verified 後の API 呼び出しで実際に返った場合だけユーザー向けに表示する

## Tree / paper snapshot cache

現在の実装済み cache は workspace list のみ。

保存しているもの:

```ts
type WorkspaceSnapshot = {
  workspaceId: string;
  name: string;
  ownerId: string;
  plan: number;
  storageUsedBytes: string;
  storageQuotaBytes: string;
  maxFileSizeBytes: string;
  maxUploadsPerDay: string;
  createdAt: string;
};
```

まだ保存していないもの:

- workspace root item
- document root item
- tree node
- `workspacePaperGroups`
- projection 済み paper
- subtree loaded state

tree / paper snapshot cache を導入すると、リロード直後に workspace の中身も前回状態で表示できる。

想定する snapshot:

```ts
type WorkspaceTreeSnapshot = {
  workspaceId: string;
  rootItemId: string;
  documentRootIds: string[];
  items: SerializedSubtreeItem[];
  fullyLoaded: boolean;
  savedAt: string;
  schemaVersion: number;
};
```

これは **表示用の前回状態** であり、API authz の根拠にはしない。

復元フロー:

1. workspace snapshot を読む
2. tree snapshot を読む
3. cached tree から workspace paper / document root paper を投影する
4. 未 verified の間は API refresh を止める
5. verified 後に `GetTree` / `GetSubtree` で fresh state に更新する

## 責務分割

### `features/auth`

責務:

- Firebase user の復元
- `signInUser()` の実行
- `listWorkspaces()` の実行順序保証
- `verifiedWorkspaceIds` の管理

やらないこと:

- tree item の cache 管理
- paper projection
- open state reconciliation の詳細

### `features/workspaces/cache`

責務:

- workspace snapshot の保存/復元
- 将来的な tree snapshot の保存/復元
- cache schema versioning

やらないこと:

- 権限判断
- backend API 呼び出し

### `features/workspaces/tree`

責務:

- tree API の呼び出し
- tree cache のメモリ表現
- subtree merge
- document root merge

制約:

- tree API 呼び出し前に verification gate を通す
- cached tree は表示には使えるが、認可済みとはみなさない

### `features/paperMap`

責務:

- open state の保存/復元
- paper-in-paper の開閉状態

やらないこと:

- workspace の権限判定
- tree API 呼び出し可否の判断

## 現行実装状況

実装済み:

- workspace snapshot cache
- user scope ごとの workspace list 復元
- `signInUser()` -> `listWorkspaces()` の直列化
- `verifiedWorkspaceIds` の導入
- `useWorkspaceTree` の tree API gate
- 新規 workspace / root upload の verified mark

未実装:

- tree snapshot cache
- paper projection snapshot cache
- `rejected` state の明示モデル
- `not_listed` workspace の穏やかな close / quarantine UI
- cache schema migration
- cache eviction policy

## 実装フェーズ

### Phase 1: API gate と workspace snapshot

目的:

- 403 race を止める
- リロード直後の workspace list 表示速度を維持する

実装:

- `workspaceSnapshotCache`
- `verifiedWorkspaceIds`
- `canFetchWorkspaceTree`

### Phase 2: Tree snapshot cache

目的:

- リロード直後に workspace の中身も表示する

実装:

- `WorkspaceTreeSnapshot`
- `workspaceTreeCache.replaceWorkspaceTreeFromSnapshot`
- `workspaceTreeCache.saveSnapshot`
- `schemaVersion` と `savedAt`

制約:

- snapshot 由来 tree は `cached` として扱う
- verified までは refresh しない

### Phase 3: Rejected / quarantine state

目的:

- cache と authoritative list の不一致をユーザーに自然に見せる

実装:

- `WorkspaceVerificationState`
- `not_listed` / `permission_denied` / `deleted` の分類
- open state からの close
- cache eviction policy

### Phase 4: Observability

目的:

- deploy 後・reload 後の race を検知しやすくする

ログ:

- cached workspace count
- verified workspace count
- gate により抑止された tree API count
- rejected workspace count
- `PermissionDenied` が verified 後に発生したかどうか

## 不変条件

- cache は権限の根拠にしない
- `GetTree` / `GetSubtree` は verified workspace だけで呼ぶ
- `PermissionDenied` を cache reconciliation の通常経路としてユーザー表示しない
- `signInUser()` より前に `listWorkspaces()` を呼ばない
- workspace list と tree/paper snapshot は別レイヤとして扱う
