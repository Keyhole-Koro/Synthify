import { getApps, initializeApp, type App } from 'firebase-admin/app';
import { getAuth, type Auth, type DecodedIdToken } from 'firebase-admin/auth';
import { config } from '../config';

// firebase-admin は FIREBASE_AUTH_EMULATOR_HOST が設定されていれば自動でエミュ
// レータに接続し、署名検証をスキップする。本番では ADC を利用するが、ID トークン
// の検証は projectID と Google の公開鍵だけで完結するため service account 鍵は
// 不要 (apps/api の FirebaseAuthenticator と同じ前提)。
function getAdminApp(): App {
  const apps = getApps();
  if (apps.length > 0) return apps[0]!;
  return initializeApp({ projectId: config.FIREBASE_PROJECT_ID });
}

export function adminAuth(): Auth {
  return getAuth(getAdminApp());
}

export type { DecodedIdToken };
