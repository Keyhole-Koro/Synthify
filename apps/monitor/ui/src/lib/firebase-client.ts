'use client';

import { getApp, getApps, initializeApp, type FirebaseApp } from 'firebase/app';
import { connectAuthEmulator, getAuth, type Auth } from 'firebase/auth';
import { authEmulatorUrl, firebaseConfig } from '@/config.client';

let cachedAuth: Auth | null = null;

export function getFirebaseAuth(): Auth {
  if (cachedAuth) return cachedAuth;

  const app: FirebaseApp = getApps().length > 0 ? getApp() : initializeApp(firebaseConfig);
  const auth = getAuth(app);

  if (authEmulatorUrl) {
    // HMR / 再評価で二重接続すると Firebase が例外を投げるため window フラグで一度だけ。
    const win = window as Window & { _monitorAuthEmulatorConnected?: boolean };
    if (!win._monitorAuthEmulatorConnected) {
      try {
        connectAuthEmulator(auth, authEmulatorUrl, { disableWarnings: true });
      } catch {
        // 二重接続などは無視して続行する。
      }
      win._monitorAuthEmulatorConnected = true;
    }
  }

  cachedAuth = auth;
  return auth;
}
