/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly MODE: 'development' | 'production' | 'test';
  readonly NEXT_PUBLIC_API_BASE_URL: string;
  readonly NEXT_PUBLIC_FIREBASE_API_KEY: string;
  readonly NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN: string;
  readonly NEXT_PUBLIC_FIREBASE_PROJECT_ID: string;
  readonly NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET: string;
  readonly NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID: string;
  readonly NEXT_PUBLIC_FIREBASE_APP_ID: string;
  readonly NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL?: string;
  readonly NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST?: string;
  readonly NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT?: string;
  readonly NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY?: string;
  readonly NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID?: string;
  readonly NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID?: string;
  readonly NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY?: string;
  readonly NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID?: string;
  readonly NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON?: string;
  readonly NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
