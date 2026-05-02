# 依存関係アーキテクチャ・ガイドライン

## 概要

Synthify プロジェクトにおける各モジュールの責務、依存の方向、および契約（Contract）の管理方法を定義します。本アーキテクチャは「変更しやすく、壊れにくく、生成物がずれにくい」状態を維持することを目的としています。

---

## 1. 依存の基本原則

### 1.1 一方向依存の徹底（Dependency Rule）
依存は常に「外側から内側へ」向かう必要があります。

*   **内側（Core/Contract）:** `packages/shared`, `packages/proto-ts`
*   **外側（Implementation/App）:** `apps/api`, `apps/worker`, `apps/log-viewer`, `apps/web`

**禁止事項:**
*   `shared` から `api` や `worker` を import すること。
*   `shared` が特定のデータベース実装や外部サービスの具象 SDK に強く依存すること。

### 1.2 契約と実装の分離
共通層（`shared`）は「何をするか（Interface）」のみを定義し、「どうやるか（Implementation）」は各アプリケーション層で実装します。

---

## 2. プロジェクトの物理構造と実依存

### 2.1 プロジェクト構造マップ

プロジェクトは以下の階層構造で管理されています。

```text
[Synthify root / superproject]
├─ Git submodules
│  ├─ apps/api
│  ├─ apps/worker
│  ├─ apps/web
│  │  └─ nested submodule: vender/paper-in-paper
│  ├─ apps/log-viewer
│  └─ packages/shared
│
├─ JS/TS workspace (Bun)
│  ├─ apps/web
│  ├─ apps/log-viewer/ui
│  └─ packages/proto-ts
│
├─ Go module
│  └─ github.com/synthify/backend
│     └─ replace github.com/synthify/backend/packages/shared
│        => ./packages/shared
│
└─ Shared packages
   ├─ packages/shared
   │  └─ Go 共通コード + Go generated proto
   └─ packages/proto-ts
      └─ TS generated proto
```

### 2.2 コード上の実依存関係

各エコシステムにおける実際の依存関係は以下の通りです。

#### Go 系
```text
api -------------> packages/shared
worker ----------> packages/shared
root go.mod -----> packages/shared

apps/log-viewer -> packages/shared
  ※ apps/log-viewer/go.mod で ../../packages/shared を replace
```

#### Web 系
```text
apps/web --------> packages/proto-ts
apps/log-viewer/ui
                -> packages/proto-ts

apps/web --------> vender/paper-in-paper
```

---

## 3. モジュール別の責務

### 3.1 `proto/` (Source of Truth)
API 契約の唯一のソースです。Connect / gRPC のスキーマを管理します。
*   API の変更は必ずここから始まります。
*   `buf generate` により、Go と TypeScript のコードが自動生成されます。

### 3.2 `packages/shared` (Go Contract Layer)
Go バックエンド全体の共通契約層です。
*   **保有するもの:** Domain Types, Repository Interfaces, Middleware, Config, `joblog` 契約, 生成された Go Proto コード。
*   **保有しないもの:** UI 関連、特定の DB ドライバに依存したクエリ実装。

### 3.3 `packages/proto-ts` (TS Contract Layer)
フロントエンド全体の共通契約層です。
*   **保有するもの:** 生成された TypeScript/Connect-ES クライアント。
*   **メリット:** `web` と `log-viewer/ui` で同一の型定義・クライアントを共有し、契約のズレを防ぎます。

### 3.4 `apps/` (Applications)
*   **api:** HTTP エントリポイント。ユースケースのオーケストレーション。
*   **worker:** 非同期ジョブ実行、LLM 連携。
*   **web / log-viewer/ui:** プロダクトのフロントエンド実装。

---

## 4. 抽象化パターン: `joblog`


依存の逆流を防ぐための標準的なパターンです。

1.  **契約 (shared/joblog):** `Logger` インターフェースと `Event` 構造体を定義。
2.  **実装 (apps/log-viewer):** `Logger` を実装し、DB への保存等を行う。
3.  **利用 (api/worker):** `shared/joblog` のインターフェースのみを使い、実装には関知しない。

これにより、ログの保存先を Stdout から Firestore や Postgres に変更しても、ビジネスロジック（api/worker）を修正する必要がなくなります。

---

## 4. 開発ワークフロー

### 4.1 API 変更の手順
1.  `proto/` 内の `.proto` ファイルを編集。
2.  ルートディレクトリで `buf generate` を実行。
3.  `packages/shared/gen` と `packages/proto-ts/gen` が更新されたことを確認。
4.  コンパイルエラー（破壊的変更）を各アプリで修正。

### 4.2 フロントエンドの依存管理
Bun Workspaces を使用しています。
*   `packages/proto-ts` を更新したら、`bun install` を実行してワークスペース内の依存を解決してください。
*   共通ロジックを切り出す場合は `packages/` 下に新規パッケージを作成することを検討してください。

---

## 5. 過去の意思決定（ADR 短記）

*   **Git Submodule の廃止:** モジュール間の同期コストが高すぎたため、モノリポジトリ構成へ移行しました。
*   **Shared の一本化:** `shared-types`, `shared-utils` のように細分化せず、Go Package 境界で管理することで、プロジェクト全体の認知負荷を下げています。
*   **Proto-TS の独立パッケージ化:** Web と Log Viewer でコード重複が発生し、API 変更時に片方が壊れる問題を防ぐために導入しました。
