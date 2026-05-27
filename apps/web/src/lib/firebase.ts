import { initializeApp, getApps, getApp } from 'firebase/app';
import { getAuth, connectAuthEmulator } from 'firebase/auth';
import { getFirestore, connectFirestoreEmulator } from 'firebase/firestore';
import { env } from '@/config/env';

const app = !getApps().length ? initializeApp(env.firebase) : getApp();
const auth = getAuth(app);
const db = getFirestore(app);

// Connect only when the emulator URL/host is configured.
// Branch on configuration presence instead of nodeEnv so production Firebase
// can still be used in production builds by simply leaving
// NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL unset.
if (env.firebase.authEmulatorUrl) {
  connectAuthEmulator(auth, env.firebase.authEmulatorUrl, { disableWarnings: true });
}
if (env.firebase.firestoreEmulatorHost && env.firebase.firestoreEmulatorPort) {
  connectFirestoreEmulator(db, env.firebase.firestoreEmulatorHost, env.firebase.firestoreEmulatorPort);
}

export { app, auth, db };
