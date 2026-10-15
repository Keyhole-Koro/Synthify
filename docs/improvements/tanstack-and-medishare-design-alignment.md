# Synthify コードデザイン刷新 & TanStack 移行計画

## 1. 背景と目的

[MediShare](https://github.com/Keyhole-Koro/MediShare) で培われた以下のハイレベルなコードデザイン原則を、[Synthify](https://github.com/Keyhole-Koro/Synthify) に適用する。
また、実質的に SPA 静的エクスポート (`output: 'export'`) として使っていた Next.js を廃止し、**Vite + TanStack Router + TanStack Query** に刷新する。

### MediShare で追求されたコードデザインの 5 つの柱
1. **モジュラー境界と依存の機械的強制**: Feature 単位での責務分離（client / shared / dto）と ESLint `no-restricted-imports` による依存方向の厳格な統制。
2. **TanStack Router × TanStack Query の型安全・宣言的結合**: 薄いファイルルート、`beforeLoad` / `loader` でのプリフェッチと認証ガード、`queryOptions` によるクエリとキャッシュの一元管理。
3. **完全型安全なエラーコード体系と自己記述型エラー**: `ERROR_CODES` 定数、文字列依存のない UI エラー分岐、`x-request-id` による診断情報の追跡。
4. **セキュリティヘッダーの一元管理と同期検証**: TypeScript による CSP 等の定義と Hosting 設定の自動同期・CI ドリフト検証。
5. **テストのドメイン垂直統合と可視化基盤**: ドメインごとのテスト配置と、UI/通知プレビュー等の視覚的検証。

---

## 2. 移行スコープとロードマップ

### Phase 1: Vite + TanStack Router への移行（Next.js の完全撤廃）
- **依存パッケージ刷新**:
  - 削除: `next`
  - 追加: `@tanstack/react-router`, `@tanstack/react-query`, `@tanstack/router-plugin`, `@fontsource-variable/noto-sans-jp`
  - 開発依存: `vite`, `@vitejs/plugin-react`
- **Vite 設定**: `apps/web/vite.config.ts`（ポート 5173、ビルド出力先 `out`、TanStack Router プラグイン、パスエイリアス `@`）
- **HTML & エントリ**: `apps/web/index.html`, `apps/web/src/client/main.tsx`
- **ルーター構築**:
  - `src/client/router.tsx`: `createAppRouter`, `RouterContext`, `queryClient`
  - `src/client/routes/__root.tsx`: ルートレイアウト（`NewRelicUserSync`, エラーバウンダリ, 404）
  - `src/client/routes/index.tsx`: `/` -> `LandingPageView`
  - `src/client/routes/view.$slug.tsx`: `/view/*` -> `ShareLinkPageClient`
- **Next 依存の解消**:
  - `LandingPageView.tsx`: `next/dynamic` を `React.lazy` + `Suspense` に置換
  - CSS / Font: `layout.tsx` の `localFont` を `@fontsource-variable/noto-sans-jp` または CSS `@font-face` に移行
- **環境調整**:
  - `compose.yaml`: frontend コンテナ起動コマンドを `bun run dev` (Vite) に更新
  - `next.config.ts`, `next-env.d.ts`, `apps/web/app/` の撤去・整理
- **ビルド・起動確認**: `bun run build`, `bun run typecheck`

### Phase 2: TanStack Query によるデータフェッチ & キャッシュ統合
- `QueryClient` を RouterContext に統合し、全ルートから参照可能にする
- Workspace 一覧、Document ツリー、Billing 情報などの ConnectRPC 取得を `queryOptions` に整理
- `useLandingPageController.ts` の過密なステート同期を TanStack Query のキャッシュ駆動にリファクタリング

### Phase 3: エラーコード体系と診断基盤（MediShare 流）
- `src/shared/error-codes.ts`: `ERROR_CODES` / `ErrorCode` 定数および型ガード `isErrorCode` を定義
- `src/lib/rpc.ts`: `x-request-id` 抽出、`ApiRequestError` での構造化エラー伝播
- UI 側のエラーメッセージ依存判定を型安全なエラーコード判定に移行

### Phase 4: ESLint によるアーキテクチャ境界の機械的強制
- `apps/web/eslint.config.js`: `no-restricted-imports` によるモジュール間・レイヤー間の参照制限ルールを追加

### Phase 5: セキュリティヘッダー一元管理 & E2E テストのドメイン垂直統合
- `apps/web/src/shared/security-headers.ts`: TypeScript による CSP・HSTS・フレーム制御ヘッダー定義
- `scripts/sync-hosting-config.ts`: `firebase.json` とのヘッダー自動同期スクリプト（`--check` 検証付き）
- `apps/web/e2e/`: ドメイン別にテストを再編（`auth/`, `billing/`, `workspaces/`, `documents/`, `sharing/`, `general/`）
- `apps/web/package.json`: ドメイン別 E2E 実行スクリプト（`e2e:auth`, `e2e:billing`, etc.）の配備

---

## 3. ディレクトリ構成（移行後）

```text
apps/web/
  index.html
  vite.config.ts
  package.json
  e2e/                         # ドメイン垂直統合 E2E テスト
    auth/                      # 認証・認可・セッション (auth.setup.ts, authorization.spec.ts, etc.)
    billing/                   # 課金・プラン・予算 (billing.spec.ts)
    workspaces/                # ワークスペース・チャット (workspace.spec.ts, etc.)
    documents/                 # アップロード・ドキュメントツリー (upload.spec.ts, paper.spec.ts)
    sharing/                   # 共有リンク (sharing.spec.ts)
    general/                   # ナイトリー・UI・スクリーンショット (nightly.spec.ts, etc.)
    fixtures/
    helpers/
  src/
    client/                    # クライアントルーティング・エントリ
      main.tsx
      router.tsx
      routeTree.gen.ts         # TanStack Router 自動生成
      routes/
        __root.tsx
        index.tsx
        view.index.tsx
        view.$token.tsx
    features/                  # ドメイン別機能モジュール (Query / Component / Reducer)
      landing/
      workspaces/
        queries.ts             # TanStack Query queryOptions
      documents/
      sharing/
      billing/
        queries.ts             # TanStack Query queryOptions
      auth/
      jobs/
      paperMap/
    shared/                    # 共通エラーコード・セキュリティヘッダー・型
      error-codes.ts
      runtime-errors.ts
### Phase 6: 宣言的ローディング、エラー診断統合、境界厳格化 & DX 向上
- **残存ディレクティブ・シムの完全清掃**: 27 箇所の `'use client';` および `eslint.config.js` の `.next` を完全除去
- **TanStack Router 宣言的プリフェッチ**: `view.$token.tsx` に `loader`（`queryClient.ensureQueryData`）を導入し、ウォーターフォールを排除
- **TanStack DevTools 配備**: 開発環境限定の lazy load DevTools（Router & Query）を導入（本番バンドルからは完全除外）
- **エラー診断パイプライン統合**: `toAppError` が `ApiRequestError` / `RuntimeError` の `ErrorCode` や `requestId` を解釈・伝達するよう結合
- **ESLint モジュラー境界ルール強化**:
  - `src/features/**`: `@/client/*` からの import を禁止（機能モジュールがルーティング具象に逆依存しない）
  - `src/components/**`: `@/features/*` からの import を禁止（共通 UI コンポーネントが個別ドメインに依存しない）

---

## 3. ディレクトリ構成（移行後）

```text
apps/web/
  index.html
  vite.config.ts
  package.json
  e2e/                         # ドメイン垂直統合 E2E テスト
    auth/                      # 認証・認可・セッション (auth.setup.ts, authorization.spec.ts, etc.)
    billing/                   # 課金・プラン・予算 (billing.spec.ts)
    workspaces/                # ワークスペース・チャット (workspace.spec.ts, etc.)
    documents/                 # アップロード・ドキュメントツリー (upload.spec.ts, paper.spec.ts)
    sharing/                   # 共有リンク (sharing.spec.ts)
    general/                   # ナイトリー・UI・スクリーンショット (nightly.spec.ts, etc.)
    fixtures/
    helpers/
  src/
    client/                    # クライアントルーティング・エントリ
      main.tsx
      router.tsx
      Devtools.tsx             # 開発環境用 TanStack Router / Query DevTools
      NewRelicUserSync.tsx     # クライアント監視同期
      routeTree.gen.ts         # TanStack Router 自動生成
      routes/
        __root.tsx
        index.tsx
        view.index.tsx
        view.$token.tsx
    features/                  # ドメイン別機能モジュール (Query / Component / Reducer)
      landing/
      workspaces/
        queries.ts             # TanStack Query queryOptions
      documents/
      sharing/
      billing/
        queries.ts             # TanStack Query queryOptions
      auth/
      jobs/
      paperMap/
    shared/                    # 共通エラーコード・セキュリティヘッダー・型
      error-codes.ts
      runtime-errors.ts
      security-headers.ts
    lib/                       # 通信・Firebase・New Relic 等の基盤
      rpc.ts                   # x-request-id 伝播・構造化エラーハンドリング
      errors.ts                # AppError (requestId 診断トレース対応)
      error_normalize.ts       # 統合エラー正規化パイプライン
      connect.ts
      firebase.ts
      firestore/
```

---

## 4. 移行完了ステータス

| 項目 | 検証コマンド | 結果 |
| :--- | :--- | :--- |
| **TypeScript 型検査** | `bun run web:typecheck` | PASS (0 errors) |
| **ESLint 境界検査** | `bun run web:lint` | PASS (0 errors, 0 warnings) |
| **Vitest ユニット/統合テスト** | `bun run web:test` | PASS (12 files, 61 tests passed) |
| **Vite プロダクションビルド** | `bun run web:build` | PASS (1.15s, `apps/web/out` 出力) |
| **セキュリティヘッダー同期** | `bun run check:hosting` | PASS (firebase.json is up to date) |
| **E2E テスト探索** | `bunx playwright test --list` | PASS (14 files, 30 tests correctly matched) |
