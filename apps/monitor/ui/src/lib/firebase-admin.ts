/**
 * Firebase Admin SDK initialization for server-side token verification.
 *
 * @module
 */
import { getApps, initializeApp, type App } from 'firebase-admin/app';
import { getAuth, type Auth, type DecodedIdToken } from 'firebase-admin/auth';
import { config } from '../config';

/**
 * Returns the existing initialized Firebase Admin App singleton or creates a new one.
 *
 * @returns Firebase Admin app configured with the monitor project ID.
 * @throws Firebase Admin initialization errors when server credentials/configuration are invalid.
 */
function getAdminApp(): App {
  const apps = getApps();
  if (apps.length > 0) return apps[0]!;
  return initializeApp({ projectId: config.FIREBASE_PROJECT_ID });
}

/**
 * Obtains the Firebase Admin Auth service instance for verifying ID tokens.
 *
 * @returns Cached Admin Auth service bound to the monitor Firebase project.
 * @throws Firebase Admin initialization errors when the app cannot be created.
 */
export function adminAuth(): Auth {
  return getAuth(getAdminApp());
}

export type { DecodedIdToken };
