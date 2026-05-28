# Frontend Observability with New Relic

## 現状の課題
- 現在、バックエンド (API, Worker) には New Relic APM が導入されているが、フロントエンド (Next.js) のエラーやパフォーマンスデータは収集されていない。
- `apps/web/app/error.tsx` にはロギングの必要性がコメントされているが、具体的な実装は未着手である。
- フロントエンド特有のエラー（JavaScript 実行エラー、ネットワークエラー、UI のレンダリング遅延など）が可視化されていない。

## 改善目標
1. **フロントエンドエラーの捕捉**: クライアントサイドで発生した実行エラーを New Relic Browser に自動送信する。
2. **パフォーマンスモニタリング**: Core Web Vitals (LCP, FID, CLS) やリソースの読み込み時間を計測し、ユーザー体験を数値化する。
3. **分散トレーシングの統合**: フロントエンドからバックエンドへのリクエストにトレース ID を付加し、一連の流れを New Relic 上で紐付けて追跡可能にする。

## 実装計画

### Phase 1: New Relic Browser エージェントの導入
- **環境変数の追加**: `.env` および `.env.example` に必要なキーを追加する。
  - `NEXT_PUBLIC_NEW_RELIC_APP_ID`
  - `NEXT_PUBLIC_NEW_RELIC_LICENSE_KEY`
  - `NEXT_PUBLIC_NEW_RELIC_ACCOUNT_ID`
  - `NEXT_PUBLIC_NEW_RELIC_TRUST_KEY`
- **インジェクション・コンポーネントの作成**: New Relic のブラウザ用 JavaScript スニペットを安全に注入するためのコンポーネントを作成。
- **Layout への組み込み**: `apps/web/app/layout.tsx` の `<head>` 内で上記コンポーネントを呼び出す。

### Phase 2: エラーハンドリングの強化
- **Error Boundary との連携**: `apps/web/app/error.tsx` および `global-error.tsx` で `newrelic.noticeError(error)` を呼び出し、捕捉したエラーを明示的に送信する。
- **カスタム属性の付与**: ログイン中のユーザー ID やワークスペース ID を属性として付加し、エラー発生時のコンテキストを特定しやすくする。

### Phase 3: 分散トレーシング (Distributed Tracing) の設定
- **ConnectRPC Options**: `apps/web/src/lib/api-client.ts` (存在する場合) 等の通信クライアント設定で、W3C Trace Context ヘッダーを送信するように構成する。
- **CORS 設定の確認**: バックエンド側で New Relic のトレース用ヘッダー (`traceparent`, `tracestate`) を許可するように `CORS_ALLOWED_ORIGINS` と関連設定を調整する。

## 実装のヒント (Next.js)
Next.js 13+ (App Router) では、`next/script` を使用して以下のようにエージェントを読み込むのが一般的です。

```tsx
// apps/web/components/NewRelicAgent.tsx
'use client';

import Script from 'next/script';

export const NewRelicAgent = () => {
  return (
    <Script
      id="new-relic-agent"
      strategy="afterInteractive"
      dangerouslySetInnerHTML={{
        __html: `
          // New Relic Browser Agent Snippet from Dashboard
          window.NREUM||(NREUM={});...
        `,
      }}
    />
  );
};
```

## 関連ドキュメント
- [../architecture/environment-variables.md](../architecture/environment-variables.md) (環境変数の管理)
- [apps/web/app/error.tsx](../../apps/web/app/error.tsx) (現在のエラー境界実装)
