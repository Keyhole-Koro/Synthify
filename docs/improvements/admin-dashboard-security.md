# Admin Dashboard Security & Management Features

## 現状の課題
- `log-viewer` (将来の管理者用 Dashboard) の API エンドポイント (`/api/jobs`, `/api/dashboards/*`) が無認証で公開されており、誰でも全ユーザーの活動履歴やコスト情報を閲覧できてしまう。
- クレジット付与 (`GrantCredit`) などの特権操作を実行する UI が存在せず、現在は API 直叩きが必要。

## 改善目標
1. **Dashboard の保護**: 管理者以外がアクセスできないように認証・認可を導入する。
2. **管理者機能の統合**: Dashboard からクレジット付与やユーザー管理などの特権操作を実行可能にする。

## 実装計画

### Phase 1: Dashboard API の保護
- **認証の導入**: `log-viewer` の Next.js Route Handlers に Firebase Auth 等による認証チェックを追加。
- **認可の導入**: `SYNTHIFY_ADMIN_USER_EMAILS` に含まれるユーザーのみが API を実行できるように制限。
- **ミドルウェア化**: `src/middleware.ts` を活用し、Dashboard 全体へのアクセスを管理者限定にする。

### Phase 2: 管理者操作 UI の実装
- **Credit Management UI**:
  - 特定の Account ID に対して金額と理由を入力して送信するフォームを実装。
  - API サーバーの `GrantCredit` (ConnectRPC) を呼び出す Backend Proxy を Dashboard 側に作成。
- **User/Workspace Lookup**:
  - 特定のユーザーや Workspace を検索・特定し、活動状況をドリルダウンできる機能。

### Phase 3: セキュアなサービス間通信
- **API サーバーとの連携**: Dashboard から API サーバーへのリクエストに、管理者権限を持つ Firebase ID Token またはサービス間トークンを付与する。
- **Worker サービスの保護**: Worker 側のエンドポイントも API サーバーからのリクエスト（サービス間トークン）のみを受け付けるように多層防御を強化。

## 関連ドキュメント
- [log-viewer-bi-dashboards.md](log-viewer-bi-dashboards.md) (BI機能としての詳細)
- [../contracts/api-authorization-contract.md](../contracts/api-authorization-contract.md) (API認可モデル)
