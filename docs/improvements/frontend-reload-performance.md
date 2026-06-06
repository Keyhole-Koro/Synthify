# Frontend Reload Performance Tuning

## 現状の課題

- `apps/web` は Next.js の static export で Firebase Hosting に配信されているため、リロード直後の体感速度は SSR ではなく、静的 HTML 後の hydration、Firebase Auth 復元、`signInUser`、`listWorkspaces`、tree API、Firestore listener 初期化の順序に左右される。
- `useAuthState` は `workspaceSnapshotCache` で workspace 一覧を `localStorage` から即時復元しているが、tree/paper の復元はメモリ内 `workspaceTreeCache` に閉じている。リロードすると開いていた workspace paper は先に空の paper として表示され、tree は `getTree` の完了後に再投影される。
- `usePersistentPaperOpenState` は open state / focus を永続化しているため、前回開いていた workspace をリロード時に再現できる。一方で、その open state に対応する tree/paper 内容は永続化されていないため、見た目だけ復元されて内容が遅れて埋まる。
- `signInUser` と `listWorkspaces` は認可前提のため直列化されている。この直列化は正しいが、画面表示のクリティカルパスに乗ると、Cloud Run cold start や token refresh 時に初期表示が遅く感じられる。
- `@keyhole-koro/paper-in-paper`、Firebase、ConnectRPC、生成済み protobuf、New Relic Browser が同じ初期 bundle に乗る可能性があり、hydration の CPU 時間もリロード体感に影響する。

## 改善目標

1. **リロード直後の即時表示**: 認証復元や API 完了を待たず、前回表示していた workspace 一覧、open state、workspace paper の最低限の内容を表示する。
2. **認可安全な再検証**: キャッシュ表示は optimistic に行うが、tree API / Firestore subscribe は `signInUser` と `listWorkspaces` で workspace が verified された後だけ実行する。
3. **表示の段階化**: 初期表示、workspace shell、cached tree、fresh tree、Firestore job status を分け、遅い処理が早い表示をブロックしないようにする。
4. **計測可能な改善**: Web Vitals だけでなく、アプリ固有の reload milestones を New Relic Browser に送ってボトルネックを特定できるようにする。

## 推奨方針

### 1. Cache-first reload model を明示化する

既存の `workspaceSnapshotCache` を拡張し、workspace 一覧だけでなく「最後に表示していた workspace paper shell」と「tree root の snapshot」を保存する。

- 保存対象:
  - workspace 一覧: 現行 `workspaceSnapshotCache` を継続。
  - open state / focus: 現行 `expansionPersistence` を継続。
  - workspace paper shell: workspaceId、name、hasTree、rootContent の短い preview、updatedAt。
  - tree snapshot: root node と、開いていた node の 1 depth 分まで。巨大 tree 全体は保存しない。
- 保存タイミング:
  - `refreshWorkspaceTree` 成功時。
  - `loadSubtree` 成功時。ただし保存量は上限を設ける。
  - job が `treeChanged` を通知して fresh tree を取得した後。
- 表示ルール:
  - `localStorage` / IndexedDB から読み込んだ snapshot は `stale` として表示する。
  - verified 前は cached paper の描画のみ許可し、API refresh は開始しない。
  - verified 後に fresh tree を取得し、差し替え時に focus と expansion を維持する。

### 2. tree cache をリロード耐性のある層に分ける

現在の `workspaceTreeCache` はメモリ内 store としては適切だが、リロードでは消える。永続化層は store 本体に混ぜず、snapshot adapter として追加する。

- `workspaceTreeSnapshotCache.ts` を追加し、protobuf message ではなく軽量 JSON で保存する。
- `createWorkspaceTreeCache()` は引き続きメモリ store として扱う。
- `useWorkspaceTree` の初期化時に snapshot を cache に hydrate するための `restoreWorkspaceTreeSnapshot(workspaceId)` を用意する。
- snapshot から復元した workspace は `initialized` 扱いにするが、`fullyLoaded` 扱いにはしない。fresh `getTree` が成功した時だけ `fullyLoaded` にする。

これにより、リロード直後は cached subtree を表示しつつ、バックグラウンドで最新 tree に置き換えられる。

### 3. verified gate と background refresh を分離する

`useAuthState` の `verifiedWorkspaceIds` は重要な安全装置なので維持する。改善点は、verified 前に UI を止めるのではなく、verified 後に実行する作業を queue 化すること。

- open state に含まれる workspaceId を `pendingRefreshWorkspaceIds` として保持する。
- `verifiedWorkspaceIds` に追加された workspace だけ `refreshWorkspaceTree` を開始する。
- 同時 refresh 数は 2 件程度に制限する。大量 workspace のリロードで API と CPU を詰まらせない。
- ユーザーが明示的に開いた workspace は queue の先頭に上げる。
- `listWorkspaces` に失敗した場合でも cached workspace shell は表示し、verified されていないことを UI 状態で区別する。

### 4. 初期 bundle と hydration を軽くする

static export では初期 JS の量が直接リロード体感に出る。初期画面に不要な機能は遅延読み込みする。

- `@keyhole-koro/paper-in-paper` の canvas 本体を `next/dynamic` で client-only lazy load できるか検証する。
- billing panel、upload dropzone、New Relic user sync、debug API、Firestore job list など、初期 LCP に不要な部分は workspace paper が開いた後に読み込む。
- 生成済み protobuf は service ごとの import 境界を保ち、landing 初期表示で admin / monitor 系 proto が混入しないよう bundle analyzer で確認する。
- New Relic Browser は計測価値があるが、agent 読み込みが初期表示を圧迫する場合は `afterInteractive` 相当の読み込み順にする。

### 5. Firestore listener の開始を遅らせる

`WorkspaceJobList` や job progress は重要だが、リロード直後の first paint をブロックすべきではない。

- workspace paper shell は job listener なしで先に描画する。
- Firestore `onSnapshot` は、workspace が verified 済みかつ paper が実際に開かれてから subscribe する。
- listener 初期化は tree refresh と同時に大量起動しないよう、workspace ごとに遅延または優先度を付ける。
- cached job status が必要なら、最新 3 件程度の last-known status を別 snapshot として保存する。

## 実装計画

### Phase 1: 計測とボトルネック特定

- reload milestone を追加する。
  - `app_hydrated`
  - `auth_state_restored`
  - `workspace_snapshot_rendered`
  - `workspaces_verified`
  - `cached_tree_rendered`
  - `fresh_tree_rendered`
  - `job_listener_ready`
- New Relic Browser に custom event / custom attribute として送る。
- `next build` の bundle analyzer を一時的に有効化し、landing route の初期 bundle に含まれる大きい依存を確認する。

### Phase 2: workspace paper / tree snapshot cache

- `workspaceTreeSnapshotCache.ts` を追加する。
- snapshot schema version を持たせ、破壊的変更時に古い cache を無視できるようにする。
- `refreshWorkspaceTree` / `loadSubtree` 成功時に snapshot を保存する。
- `useWorkspaceTree` 起動時、open state に含まれる workspace だけ snapshot を復元して `setWorkspacePapers` する。
- verified 後に fresh refresh を走らせ、snapshot と API 結果の差し替えを確認するテストを追加する。

### Phase 3: refresh queue と listener 遅延

- verified workspace refresh queue を導入する。
- 同時 refresh 数、ユーザー操作時の優先順位、失敗時の retry/backoff を定義する。
- Firestore listener は opened + verified 条件で開始する。
- リロード時に 10 workspace 以上あるケースの手動確認を行う。

### Phase 4: bundle 分割

- `paper-in-paper` canvas、billing、upload、job list、debug-only code の lazy boundary を検討する。
- bundle analyzer で before / after を比較する。
- LCP と input readiness が悪化していないことを確認する。

## 受け入れ基準

- ログイン済みユーザーのリロードで、workspace 一覧と前回 focus していた workspace shell が API 完了前に表示される。
- verified 前に tree API / Firestore workspace listener が実行されない。
- verified 後、cached tree が fresh tree に置き換わっても focus / expansion が壊れない。
- `listWorkspaces` が一時的に失敗しても、cached 表示は残り、再試行で fresh state に復帰できる。
- reload milestone が New Relic か console fallback で確認できる。

## 関連ファイル

- [workspace-cache-verification-model.md](workspace-cache-verification-model.md)
- [frontend-observability-newrelic.md](frontend-observability-newrelic.md)
- [apps/web/src/features/auth/useAuthState.ts](../../apps/web/src/features/auth/useAuthState.ts)
- [apps/web/src/features/workspaces/cache/workspaceSnapshotCache.ts](../../apps/web/src/features/workspaces/cache/workspaceSnapshotCache.ts)
- [apps/web/src/features/workspaces/useWorkspaceTree.ts](../../apps/web/src/features/workspaces/useWorkspaceTree.ts)
- [apps/web/src/features/workspaces/tree/workspaceTreeCache.ts](../../apps/web/src/features/workspaces/tree/workspaceTreeCache.ts)
- [apps/web/src/features/paperMap/hooks/usePersistentPaperOpenState.ts](../../apps/web/src/features/paperMap/hooks/usePersistentPaperOpenState.ts)
