import { defineConfig, devices } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import baseConfig from './playwright.config';

// Config for recording UI demo clips (e2e/*.demo.ts). Separate from the test
// config because the goals conflict: the suite records video only on failure
// and runs in parallel, while a demo needs video always, one worker, and a
// fixed viewport so the clip is a predictable size.
//
// The stack is assumed to be already running:
//   docker compose up -d frontend backend worker
//
//   npx playwright test e2e/stif-import.demo.ts --config=playwright.demo.config.ts
const authFile = fileURLToPath(new URL('./e2e/.auth/user.json', import.meta.url));

export default defineConfig({
  ...baseConfig,
  testDir: './e2e',
  testMatch: /\.(demo|setup)\.ts$/,
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'list',
  outputDir: './demo-results',
  use: {
    ...baseConfig.use,
    video: { mode: 'on', size: { width: 1280, height: 800 } },
    viewport: { width: 1280, height: 800 },
    trace: 'off',
    screenshot: 'off',
  },
  projects: [
    { name: 'setup', testMatch: /auth\.setup\.ts/ },
    {
      name: 'chromium',
      testMatch: /\.demo\.ts$/,
      use: { ...devices['Desktop Chrome'], viewport: { width: 1280, height: 800 }, storageState: authFile },
      dependencies: ['setup'],
    },
  ],
  // The demo drives an already-running stack; booting it here would fight with
  // the compose project the operator started by hand.
  webServer: undefined,
});
