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

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
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
      use: {
        ...devices['Desktop Chrome'],
        storageState: authFile,
      },
      dependencies: ['setup'],
    },
  ],
  webServer: process.env.E2E_REUSE_SERVER
    ? undefined
    : {
        // Build the only local image explicitly: Compose v5's bake path is
        // unstable on some Docker Desktop/WSL builds.
        command: 'docker build -t synthify-dev-firebase-auth ../../docker/firebase-emulator && docker compose -f ../../compose.yaml up --no-build frontend backend worker',
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: 300_000,
        stdout: 'pipe',
        stderr: 'pipe',
      },
});
