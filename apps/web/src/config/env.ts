import { z } from 'zod';

const rawEnvSchema = z.object({
  NODE_ENV: z.enum(['development', 'test', 'production']),
  NEXT_PUBLIC_API_BASE_URL: z.string().url(),
  INTERNAL_API_BASE_URL: z.string().url().optional(),
  NEXT_PUBLIC_FIREBASE_API_KEY: z.string().min(1),
  NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN: z.string().min(1),
  NEXT_PUBLIC_FIREBASE_PROJECT_ID: z.string().min(1),
  NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET: z.string().min(1),
  NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID: z.string().min(1),
  NEXT_PUBLIC_FIREBASE_APP_ID: z.string().min(1),
  // Connect only when the emulator is configured. Unset means production Firebase.
  NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL: z.string().url().optional(),
  NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST: z.string().min(1).optional(),
  NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT: z.coerce.number().int().positive().optional(),
  // New Relic Browser. All optional — absent license/app id means the agent is
  // simply not initialized (no-op), so dev / preview builds don't break.
  NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY: z.string().min(1).optional(),
  NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID: z.string().min(1).optional(),
  NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID: z.string().min(1).optional(),
  NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY: z.string().min(1).optional(),
  NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID: z.string().min(1).optional(),
  NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON: z.string().min(1).optional(),
  NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON: z.string().min(1).optional(),
  // Force verbose console output regardless of NODE_ENV (e.g. a preview deploy
  // where NODE_ENV=production but you still want debug logs).
  NEXT_PUBLIC_DEBUG_LOGS: z.enum(['true', 'false']).optional(),
});

const rawEnv = rawEnvSchema.parse({
  NODE_ENV: process.env.NODE_ENV,
  NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
  INTERNAL_API_BASE_URL: process.env.INTERNAL_API_BASE_URL,
  NEXT_PUBLIC_FIREBASE_API_KEY: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN: process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN,
  NEXT_PUBLIC_FIREBASE_PROJECT_ID: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET: process.env.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET,
  NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  NEXT_PUBLIC_FIREBASE_APP_ID: process.env.NEXT_PUBLIC_FIREBASE_APP_ID,
  NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL: process.env.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL,
  NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST: process.env.NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST,
  NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT: process.env.NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY: process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID: process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID: process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY: process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID: process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON: process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON: process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON,
  NEXT_PUBLIC_DEBUG_LOGS: process.env.NEXT_PUBLIC_DEBUG_LOGS,
});

function normalizeBaseUrl(url: string): string {
  // Remove trailing slashes and /api suffix if present
  return url.replace(/\/+$/, '').replace(/\/api$/, '');
}

function isLocalHost(hostname: string): boolean {
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '0.0.0.0';
}

function browserReachableUrl(url: string): string {
  if (typeof window === 'undefined') return url;

  const parsed = new URL(url);
  if (isLocalHost(parsed.hostname) && window.location.hostname !== '0.0.0.0') {
    parsed.hostname = window.location.hostname;
  }
  return parsed.toString().replace(/\/+$/, '');
}

function browserReachableHost(host: string): string {
  if (typeof window === 'undefined') return host;
  if (isLocalHost(host) && window.location.hostname !== '0.0.0.0') {
    return window.location.hostname;
  }
  return host;
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

const newRelicBeacon = rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON ?? 'bam.nr-data.net';
const newRelic = {
  // Without a license key + application id the agent can't initialize, so treat
  // observability as disabled and skip console wrapping / NR calls entirely.
  enabled: Boolean(
    rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY &&
      rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID,
  ),
  beacon: newRelicBeacon,
  errorBeacon: rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON ?? newRelicBeacon,
  licenseKey: rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY,
  applicationID: rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID,
  accountID: rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID,
  trustKey: rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY,
  agentID:
    rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID ??
    rawEnv.NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID,
} as const;

const consoleLevel: LogLevel = rawEnv.NEXT_PUBLIC_DEBUG_LOGS === 'true'
  ? 'debug'
  : rawEnv.NODE_ENV === 'production'
    ? 'warn'
    : 'debug';

export const env = {
  nodeEnv: rawEnv.NODE_ENV,
  apiBaseUrl: normalizeBaseUrl(
    typeof window === 'undefined'
      ? (rawEnv.INTERNAL_API_BASE_URL ?? rawEnv.NEXT_PUBLIC_API_BASE_URL)
      : browserReachableUrl(rawEnv.NEXT_PUBLIC_API_BASE_URL)
  ),
  firebase: {
    apiKey: rawEnv.NEXT_PUBLIC_FIREBASE_API_KEY,
    authDomain: rawEnv.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN,
    projectId: rawEnv.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
    storageBucket: rawEnv.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET,
    messagingSenderId: rawEnv.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
    appId: rawEnv.NEXT_PUBLIC_FIREBASE_APP_ID,
    authEmulatorUrl: rawEnv.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL
      ? browserReachableUrl(rawEnv.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL)
      : undefined,
    firestoreEmulatorHost: rawEnv.NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST
      ? browserReachableHost(rawEnv.NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST)
      : undefined,
    firestoreEmulatorPort: rawEnv.NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT,
  },
  observability: {
    newRelic,
    // Minimum level written to the browser console. NR delivery is independent
    // of this (handled in log.ts / wrapLogger), so quieting the console in prod
    // does not lose telemetry.
    consoleLevel,
    // Mirror every console.error/warn (incl. third-party) into NR via wrapLogger
    // as the catch-all backstop. Pointless without an agent, so tie to enabled.
    wrapConsole: newRelic.enabled,
  },
} as const;
