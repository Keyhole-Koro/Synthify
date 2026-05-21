# 依存関係アーキテクチャ・ガイドライン

## 概要

Synthify プロジェクトにおける各モジュールの責務、依存の方向、および契約（Contract）の管理方法を定義します。本アーキテクチャは「変更しやすく、壊れにくく、生成物がずれにくい」状態を維持することを目的としています。

---

## 1. 依存の基本原則

### 1.1 一方向依存の徹底（Dependency Rule）
依存は常に「外側から内側へ」向かう必要があります。

*   **内側（Core/Contract）:** `internal/gen`, `packages/proto-ts`
*   **横断基盤（Platform）:** `internal/platform`
*   **外側（Implementation/App）:** `apps/api`, `apps/worker`, `apps/log-viewer`, `apps/web`

**禁止事項:**
*   `internal/platform` から `apps/api` や `apps/worker` を import すること。
*   アプリケーション間 (`apps/api` と `apps/worker` 等) で直接 import しあうこと。

### 1.2 契約と実装の分離
共通契約（`internal/gen`）は「何をするか（RPC Interface）」を定義し、具体的なドメインロジックや実装は各アプリケーション層に閉じ込めます。かつての `packages/shared` のような、ドメイン知識が漏れ出す巨大な共通パッケージは作成しません。

---

## 2. プロジェクトの物理構造と実依存

### 2.1 プロジェクト構造マップ

プロジェクトは以下の階層構造で管理されています。

```text
[Synthify root]
├─ contracts/connectrpc
│  └─ Connect / gRPC proto source of truth
│
├─ internal/
│  ├─ gen/        # Go generated proto / Connect code
│  └─ platform/   # 変化の少ない横断基盤 (applog, observability, util等)
│
├─ apps/
│  ├─ api/
│  │  └─ internal/ # API 固有の domain, service, repository, bootstrap, middleware
│  ├─ worker/
│  │  └─ pkg/worker/ # worker 固有の logic (eval から参照するため pkg 配下)
│  ├─ log-viewer/
│  └─ eval/        # worker の評価・テストツール (worker/pkg を参照)
│
└─ packages/
   └─ proto-ts/    # Web 向けの TS generated proto package
```

### 2.2 コード上の実依存関係

#### Go 系
```text
api --------> internal/gen, internal/platform
worker -----> internal/gen, internal/platform
eval -------> apps/worker/pkg/worker/..., internal/platform
log-viewer -> (自己完結したコードを使用)
```

---

## 3. モジュール別の責務

### 3.1 `internal/gen`
API 契約の唯一のソース (`contracts/connectrpc`) から生成された Go コードを管理します。
*   ビジネスロジックは持ちません。

### 3.2 `internal/platform`
変更頻度が低く、ビジネスドメインに依存しない純粋な技術基盤を提供します。
*   **applog:** 構造化ロガー
*   **observability:** New Relic 等のテレメトリ設定
*   **httpmiddleware:** recovery, logger 等の汎用ミドルウェア

### 3.3 `apps/api/internal`
API アプリケーションの全責任を持ちます。
*   **domain:** API 向けのエンティティ・値オブジェクト
*   **repository:** DB 実装 (SQLC 生成物含む)
*   **service:** ユースケース実行
*   **middleware:** Auth, CORS 等の API 固有処理

### 3.4 `apps/worker/pkg/worker`
Worker アプリケーションの全責任を持ちます。
*   LLM 連携、ジョブ実行パイプライン、カスタムツール等の実装。

---

## 4. 開発ワークフロー

### 4.1 API 変更の手順
1.  `contracts/connectrpc/` 内の `.proto` ファイルを編集。
2.  ルートディレクトリで `buf generate` を実行。
3.  `internal/gen` と `packages/proto-ts/gen` が更新されたことを確認。
4.  コンパイルエラーを各アプリで修正。

### 4.2 ドメイン知識の共有
API と Worker で同じデータベーステーブルを参照する場合でも、コード上のドメイン型や Repository 実装は共有（DRY）せず、それぞれのアプリケーション配下に定義・実装します。これにより、一方の変更が意図せず他方に影響を与えることを防ぎます。
