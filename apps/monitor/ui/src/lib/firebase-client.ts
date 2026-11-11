/**
 * Browser-side Firebase App and Auth client initialization singleton.
 *
 * @module
 */

import { getApp, getApps, initializeApp, type FirebaseApp } from 'firebase/app';
import { connectAuthEmulator, getAuth, type Auth } from 'firebase/auth';
import { authEmulatorUrl, firebaseConfig } from '@/client-config';

let cachedAuth: Auth | null = null;

/**
 * Lazily initializes and returns the singleton Firebase Auth client instance,
 * connecting to the local Auth emulator in development if configured.
 *
 * @returns Cached Firebase Auth instance. The first call initializes the app and, when
 *   configured, connects the instance to the local Auth emulator exactly once per browser.
 * @throws Firebase initialization errors when the public client configuration is invalid.
 */
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
