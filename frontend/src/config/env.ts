function readEnv(name: keyof NodeJS.ProcessEnv, fallback: string): string {
  return process.env[name] || fallback;
}

export const env = {
  nodeEnv: process.env.NODE_ENV || 'development',
  apiBaseUrl: readEnv('NEXT_PUBLIC_API_BASE_URL', 'http://localhost:8080'),
  firebase: {
    apiKey: readEnv('NEXT_PUBLIC_FIREBASE_API_KEY', 'demo-api-key'),
    authDomain: readEnv('NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN', 'demo-project.firebaseapp.com'),
    projectId: readEnv('NEXT_PUBLIC_FIREBASE_PROJECT_ID', 'demo-project'),
    storageBucket: readEnv('NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET', 'demo-project.appspot.com'),
    messagingSenderId: readEnv('NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID', '1234567890'),
    appId: readEnv('NEXT_PUBLIC_FIREBASE_APP_ID', '1:1234567890:web:1234567890'),
    authEmulatorUrl: readEnv('NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL', 'http://127.0.0.1:9099'),
  },
} as const;
