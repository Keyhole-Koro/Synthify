import { z } from 'zod';

const rawEnvSchema = z.object({
  NODE_ENV: z.enum(['development', 'test', 'production']),
  NEXT_PUBLIC_API_BASE_URL: z.string().url(),
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
});

const rawEnv = rawEnvSchema.parse({
  NODE_ENV: import.meta.env.MODE,
  NEXT_PUBLIC_API_BASE_URL: import.meta.env.NEXT_PUBLIC_API_BASE_URL,
  NEXT_PUBLIC_FIREBASE_API_KEY: import.meta.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN: import.meta.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN,
  NEXT_PUBLIC_FIREBASE_PROJECT_ID: import.meta.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET: import.meta.env.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET,
  NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID: import.meta.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  NEXT_PUBLIC_FIREBASE_APP_ID: import.meta.env.NEXT_PUBLIC_FIREBASE_APP_ID,
  NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL: import.meta.env.NEXT_PUBLIC_FIREBASE_AUTH_EMULATOR_URL,
  NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST: import.meta.env.NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST,
  NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT: import.meta.env.NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY: import.meta.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID: import.meta.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID: import.meta.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY: import.meta.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID: import.meta.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON: import.meta.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON,
  NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON: import.meta.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON,
});

/**
 * Normalizes an API base URL by stripping trailing slashes and the `/api` suffix.
 *
 * @param url - The raw base URL.
 * @returns The normalized base URL.
 */
function normalizeBaseUrl(url: string): string {
  // Remove trailing slashes and /api suffix if present
  return url.replace(/\/+$/, '').replace(/\/api$/, '');
}

/**
 * Checks if a given hostname points to a local address (localhost, 127.0.0.1, or 0.0.0.0).
 *
 * @param hostname - Hostname to test.
 * @returns True if the host is local.
 */
function isLocalHost(hostname: string): boolean {
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '0.0.0.0';
}

/**
 * Adapts local URLs to be reachable from the current browser host when running in remote/containerized environments.
 *
 * @param url - Original URL string.
 * @returns Reachable URL formatted for browser consumption.
 */
function browserReachableUrl(url: string): string {
  if (typeof window === 'undefined') return url;

  const parsed = new URL(url);
  if (isLocalHost(parsed.hostname) && window.location.hostname !== '0.0.0.0') {
    parsed.hostname = window.location.hostname;
  }
  return parsed.toString().replace(/\/+$/, '');
}

/**
 * Adapts local hostnames to match the current browser location hostname.
 *
 * @param host - Original host string.
 * @returns Reachable host for browser network requests.
 */
function browserReachableHost(host: string): string {
  if (typeof window === 'undefined') return host;
  if (isLocalHost(host) && window.location.hostname !== '0.0.0.0') {
    return window.location.hostname;
  }
  return host;
}

/**
 * Supported client-side logging levels.
 */
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

/**
 * Validated client environment configuration containing backend URLs, Firebase parameters,
 * and observability settings.
 */
export const env = {
  nodeEnv: rawEnv.NODE_ENV,
  apiBaseUrl: normalizeBaseUrl(browserReachableUrl(rawEnv.NEXT_PUBLIC_API_BASE_URL)),
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
    // Mirror every console.error/warn (incl. third-party) into NR via wrapLogger
    // as the catch-all backstop. Pointless without an agent, so tie to enabled.
    wrapConsole: newRelic.enabled,
  },
} as const;
