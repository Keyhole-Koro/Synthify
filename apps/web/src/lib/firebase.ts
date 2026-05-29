import { initializeApp, getApps, getApp, type FirebaseApp } from 'firebase/app';
import { getAuth, connectAuthEmulator, type Auth } from 'firebase/auth';
import { initializeFirestore, connectFirestoreEmulator, getFirestore, type Firestore } from 'firebase/firestore';
import { env } from '@/config/env';

let app: FirebaseApp;
let auth: Auth;
let db: Firestore;

const isBrowser = typeof window !== 'undefined';
const isEmulator = !!(env.firebase.firestoreEmulatorHost && env.firebase.firestoreEmulatorPort);

// Singleton control to prevent double initialization during Next.js HMR (Hot Module Replacement).
// Since the Firebase SDK does not allow multiple initializeApp calls for the same project ID,
// we retrieve the existing instance using getApp() if it's already initialized.
if (!getApps().length) {
  app = initializeApp(env.firebase);
  auth = getAuth(app);
  
  // Firestore Connection Settings:
  // Firestore emulators under Docker/WSL2 environments often experience unstable WebChannel 
  // (streaming) connections, leading to crashes like INTERNAL ASSERTION FAILED (ca9).
  // We force HTTP Long Polling ONLY when connecting to the emulator to ensure stability.
  // Production environments continue to use the default streaming transport for better performance.
  db = initializeFirestore(app, {
    experimentalForceLongPolling: isEmulator,
  });

  if (isBrowser) {
    const win = window as any;
    // Emulator connection attempts also fail if executed multiple times during HMR module re-evaluation.
    // We use a window-level global flag to ensure emulators are connected only once per session.
    if (!win._firebaseEmulatorsConnected) {
      if (env.firebase.authEmulatorUrl) {
        try {
          connectAuthEmulator(auth, env.firebase.authEmulatorUrl, { disableWarnings: true });
        } catch (e) {
          console.warn('[Firebase] Auth emulator connection error:', e);
        }
      }
      if (env.firebase.firestoreEmulatorHost && env.firebase.firestoreEmulatorPort) {
        try {
          connectFirestoreEmulator(db, env.firebase.firestoreEmulatorHost, env.firebase.firestoreEmulatorPort);
        } catch (e) {
          console.warn('[Firebase] Firestore emulator connection error:', e);
        }
      }
      win._firebaseEmulatorsConnected = true;
    }
  }
} else {
  // If the app is already initialized, reuse the existing instances.
  app = getApp();
  auth = getAuth(app);
  try {
    db = getFirestore(app);
  } catch {
    // Fallback in case the app exists but the Firestore instance hasn't been created yet.
    db = initializeFirestore(app, {
      experimentalForceLongPolling: isEmulator,
    });
  }
}

export { app, auth, db };
