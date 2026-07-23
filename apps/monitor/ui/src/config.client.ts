// クライアント (ブラウザ) 側の設定を一元化するモジュール。
// ここで参照するのは NEXT_PUBLIC_* のみ = ビルド時にクライアントバンドルへ
// 焼き込まれる公開値。サーバー専用の設定 (MONITOR_DATABASE_URL,
// SYNTHIFY_ADMIN_USER_EMAILS など) は config.ts (server) に置くこと。

// ブラウザ側 Firebase SDK の初期化設定。
export const firebaseConfig = {
  apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  authDomain: process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN,
  projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  storageBucket: process.env.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  appId: process.env.NEXT_PUBLIC_FIREBASE_APP_ID,
};

// dev で Auth エミュレータへ接続する URL (未設定なら本番 Firebase)。
export const authEmulatorUrl = process.env.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL ?? '';

export function isAuthEmulator(): boolean {
  return authEmulatorUrl !== '';
}

// スクリーンショット / ローカルプレビュー用の認証バイパス。
// NODE_ENV === 'production' により本番ビルドでは常に false へ畳み込まれるため、
// production で認証を素通りさせることはない (dev サーバー + 明示フラグのときだけ有効)。
export const PREVIEW_BYPASS =
  process.env.NODE_ENV !== 'production' &&
  process.env.NEXT_PUBLIC_MONITOR_PREVIEW === '1';
