/**
 * Client-side (browser) configuration constants and feature flags.
 * Consumes NEXT_PUBLIC_* environment variables that Vite bakes into the client bundle at build time
 * (the prefix is kept from the Next.js era so deploy variables did not have to be renamed).
 *
 * @module
 */

/** Browser-side Firebase client SDK initialization credentials. */
export const firebaseConfig = {
  apiKey: import.meta.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  authDomain: import.meta.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN,
  projectId: import.meta.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  storageBucket: import.meta.env.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: import.meta.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  appId: import.meta.env.NEXT_PUBLIC_FIREBASE_APP_ID,
};

/** URL for connecting to the local Firebase Auth emulator in development. */
export const authEmulatorUrl = import.meta.env.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL ?? '';

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
  !import.meta.env.PROD &&
  import.meta.env.NEXT_PUBLIC_MONITOR_PREVIEW === '1';
