/**
 * Client-side (browser) configuration constants and feature flags.
 * Consumes NEXT_PUBLIC_* environment variables baked into the client bundle at build time.
 *
 * @module
 */

/** Browser-side Firebase client SDK initialization credentials. */
export const firebaseConfig = {
  apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  authDomain: process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN,
  projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  storageBucket: process.env.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  appId: process.env.NEXT_PUBLIC_FIREBASE_APP_ID,
};

/** URL for connecting to the local Firebase Auth emulator in development. */
export const authEmulatorUrl = process.env.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL ?? '';

/**
 * Checks whether the client is configured to use the local Firebase Auth emulator.
 * @returns True when `NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL` is non-empty.
 */
export function isAuthEmulator(): boolean {
  return authEmulatorUrl !== '';
}

/**
 * Authentication bypass flag for screenshot testing and local UI previewing.
 * Always resolves to false in production builds.
 */
export const PREVIEW_BYPASS =
  process.env.NODE_ENV !== 'production' &&
  process.env.NEXT_PUBLIC_MONITOR_PREVIEW === '1';
