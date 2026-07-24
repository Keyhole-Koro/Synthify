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

const baseURL = process.env.E2E_BASE_URL ?? `http://localhost:${localFrontendPort()}`;
const authFile = fileURLToPath(new URL('./e2e/.auth/user.json', import.meta.url));
// Gate webServer readiness on the API, not the frontend: Next dev answers
// within seconds while the Go backend builds for minutes on a cold cache,
// and tests that start in that window fail on dead connections.
const apiHealthURL = `${process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080'}/health`;

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: {
    timeout: 10_000,
    // Post-deploy visual QA defaults. Freeze animations/caret so the
    // paper-in-paper frame transitions don't produce mid-flight diffs, and
    // absorb sub-pixel anti-aliasing noise between runs.
    toHaveScreenshot: {
      animations: 'disabled',
      caret: 'hide',
      maxDiffPixelRatio: 0.01,
    },
  },
  // Keep visual baselines in a stable, greppable location so CI can upload
  // freshly-captured snapshots as an artifact for a human to commit.
  snapshotPathTemplate: '{testDir}/__screenshots__/{testFileName}/{arg}{ext}',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // Next dev + the Go services share one local machine; unbounded browser
  // workers can starve page compilation and cause navigation timeouts.
  workers: process.env.CI ? 1 : 2,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'html',
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'setup', testMatch: /auth\.setup\.ts/ },
    {
      name: 'chromium',
      // Visual specs run in their own project against a live deploy — keep them
      // out of the emulator-backed suite.
      testIgnore: /\.visual\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        storageState: authFile,
      },
      dependencies: ['setup'],
    },
    {
      // Post-deploy visual QA: runs against E2E_BASE_URL (a live stage/prod
      // deploy), unauthenticated, with no emulator setup dependency. The fixed
      // Desktop Chrome viewport keeps screenshots deterministic across runs.
      name: 'visual',
      testMatch: /\.visual\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: process.env.E2E_REUSE_SERVER
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
