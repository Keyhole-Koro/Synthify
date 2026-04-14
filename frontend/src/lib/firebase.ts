import { initializeApp, getApps, getApp } from 'firebase/app';
import { getAuth, connectAuthEmulator } from 'firebase/auth';
import { env } from '@/config/env';

const app = !getApps().length ? initializeApp(env.firebase) : getApp();
const auth = getAuth(app);

if (env.nodeEnv === 'development') {
  // Connect to the Firebase Authentication Emulator if running locally
  connectAuthEmulator(auth, env.firebase.authEmulatorUrl!, { disableWarnings: true });
}

export { app, auth };
