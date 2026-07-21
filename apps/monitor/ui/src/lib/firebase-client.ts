'use client';

import { getApp, getApps, initializeApp, type FirebaseApp } from 'firebase/app';
import { connectAuthEmulator, getAuth, type Auth } from 'firebase/auth';

const firebaseConfig = {
  apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  authDomain: process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN,
  projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  storageBucket: process.env.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  appId: process.env.NEXT_PUBLIC_FIREBASE_APP_ID,
};

let cachedAuth: Auth | null = null;

export function getFirebaseAuth(): Auth {
  if (cachedAuth) return cachedAuth;

  const app: FirebaseApp = getApps().length > 0 ? getApp() : initializeApp(firebaseConfig);
  const auth = getAuth(app);

  const emulatorUrl = process.env.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL;
  if (emulatorUrl) {
    // HMR / 再評価で二重接続すると Firebase が例外を投げるため window フラグで一度だけ。
    const win = window as Window & { _monitorAuthEmulatorConnected?: boolean };
    if (!win._monitorAuthEmulatorConnected) {
      try {
        connectAuthEmulator(auth, emulatorUrl, { disableWarnings: true });
      } catch {
        // 二重接続などは無視して続行する。
      }
      win._monitorAuthEmulatorConnected = true;
    }
  }

  cachedAuth = auth;
  return auth;
}

export function isAuthEmulator(): boolean {
  return !!process.env.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL;
}
