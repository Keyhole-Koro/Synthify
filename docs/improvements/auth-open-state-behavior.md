# ログイン・ログアウト時の open state 挙動調査

`useDefaultOpenState` の初期値は loading 完了時に一度だけ決定される。
ログイン・ログアウトによる状態変化が正しく反映されるか未検証なので調査が必要。

## 調査項目

### 1. ログイン後に `workspaces` へ切り替わるか

**問題:** `useDefaultOpenState` は `loading === false` になった瞬間に一度だけ `resolved` を決定する（`resolved !== null` になると以降スキップ）。
そのため、初回ロード時に未ログイン → `auth` で初期化された後、ログインが完了しても `workspaces` に自動切替されない可能性がある。

**確認すること:**
- ログイン完了後に `user` / `workspaces` が変化したとき、`expansionMap` / `focusedNodeId` は更新されるか
- persisted state がない初回ユーザーでログインしたとき、`auth` のままになっていないか
- `useAuthState` の `loading` フラグはどのタイミングで `false` になるか（workspaces 取得完了前か後か）

**関連ファイル:**
- [useDefaultOpenState.ts](../apps/web/src/features/paperMap/hooks/useDefaultOpenState.ts)
- [useAuthState.ts](../apps/web/src/features/auth/useAuthState.ts)

---

### 2. ログアウト時の state リセット

**問題:** `page.tsx` の `handleLogout` は `clearExpansionMap()` と `setExpansionMap` / `setFocusedItemId` を手動で呼んでいる。
`useDefaultOpenState` 内部の `resolved` は `null` にリセットされないため、ログアウト後も前の state が残る可能性がある。

**確認すること:**
- ログアウト後に `expansionMap` が `auth` を開く状態にリセットされるか
- `useDefaultOpenState` が `resolved` を持ったまま次の `user=null` を受け取ったとき、再計算が走るか
- `page.tsx` の `handleLogout` と `useDefaultOpenState` の責務が二重になっていないか

**関連ファイル:**
- [page.tsx](../apps/web/app/page.tsx) の `handleLogout`
- [useDefaultOpenState.ts](../apps/web/src/features/paperMap/hooks/useDefaultOpenState.ts)

---

### 3. persisted state が意図しない branch を開く問題

**問題:** persisted state が存在する場合、`computeDefaultOpenState` は無視される。
前回セッションで `workspaces` を開いていたユーザーがログアウトして別アカウントでログインした場合、前のアカウントの `expansionMap` がそのまま使われる。

**確認すること:**
- ログアウト時に persisted state（localStorage）を消しているか
- 別アカウントでログインしたとき、前アカウントの workspace ID が expansionMap に残っていないか
- `clearExpansionMap()` の呼び出しタイミングが適切か

**関連ファイル:**
- [expansionPersistence.ts](../apps/web/src/features/paperMap/expansionPersistence.ts)
- [page.tsx](../apps/web/app/page.tsx) の `handleLogout`

---

## 修正の方向性（調査後に決定）

- ログイン後の切り替え: `resolved` をリセットして再計算させるか、`user` 変化を watch して明示的に切り替えるか
- ログアウト: `useDefaultOpenState` に `reset()` を生やして `handleLogout` から呼ぶか、`user === null` を検知して自動リセットするか
- persisted state の汚染: ログアウト時に必ず `clearExpansionMap()` を呼ぶことを保証する、または persisted state にアカウント ID を紐付けて不一致時は破棄する
