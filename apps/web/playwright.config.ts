import { defineConfig, devices } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

function localFrontendPort(): string {
  try {
    return readFileSync(new URL('../../.env', import.meta.url), 'utf8').match(/^FRONTEND_PORT=(\d+)$/m)?.[1] ?? '5173';
  } catch {
    return '5173';
  }
}

// Set only by the nightly job, which points at a deployed environment. Its
// presence is what switches the run from "boot the local stack" to "test what
// is already running out there".
const externalBaseURL = process.env.E2E_BASE_URL;
const baseURL = externalBaseURL ?? `http://localhost:${localFrontendPort()}`;
const authFile = fileURLToPath(new URL('./e2e/.auth/user.json', import.meta.url));
// Gate webServer readiness on the API, not the frontend: Next dev answers
// within seconds while the Go backend builds for minutes on a cold cache,
// and tests that start in that window fail on dead connections.
const apiHealthURL = `${process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080'}/health`;

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // Next dev + the Go services share one local machine; unbounded browser
  // workers can starve page compilation and cause navigation timeouts.
  workers: process.env.CI ? 1 : 2,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'html',
  use: {
    baseURL,
    screenshot: 'only-on-failure',
    // E2E_RECORD keeps the video and trace for passing runs too, so a UI change
    // can be watched rather than inferred from assertions. Off by default: on
    // every green CI run this would be pure storage.
    trace: process.env.E2E_RECORD ? 'on' : 'retain-on-failure',
    video: process.env.E2E_RECORD ? 'on' : 'retain-on-failure',
  },
  projects: [
    { name: 'setup', testMatch: /auth\.setup\.ts/ },
    {
      name: 'chromium',
      testIgnore: /nightly\.spec\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        storageState: authFile,
      },
      dependencies: ['setup'],
    },
    // The nightly checks hit a deployed environment, where there is no Auth
    // emulator: they get their own project so they skip the auth setup, which
    // mints its test user against 127.0.0.1:9099 and dies on a refused
    // connection. Selecting only the nightly specs does not avoid it — a
    // dependency project runs whatever --grep leaves in the projects it feeds.
    {
      name: 'nightly',
      testMatch: /nightly\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // A deployed target needs no local stack, and booting one only burns five
  // minutes waiting for a health endpoint the tests never call.
  webServer: externalBaseURL || process.env.E2E_REUSE_SERVER
    ? undefined
    : {
        // Build the only local image explicitly: Compose v5's bake path is
        // unstable on some Docker Desktop/WSL builds.
        command: 'docker build -t synthify-dev-firebase-auth ../../docker/firebase-emulator && E2E_WORKER_FIXTURE=true docker compose -f ../../compose.yaml up --no-build frontend backend worker',
        url: apiHealthURL,
        reuseExistingServer: !process.env.CI,
        timeout: 300_000,
        stdout: 'pipe',
        stderr: 'pipe',
      },
});
