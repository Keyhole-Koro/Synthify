import { expect, test } from '@playwright/test';
import { readFileSync } from 'node:fs';

function localBackendPort(): string {
  try {
    return readFileSync(new URL('../../../.env', import.meta.url), 'utf8').match(/^BACKEND_PORT=(\d+)$/m)?.[1] ?? '8080';
  } catch {
    return '8080';
  }
}

const apiBaseURL = process.env.E2E_API_BASE_URL ?? `http://127.0.0.1:${localBackendPort()}`;

test.describe('platform smoke tests', () => {
  test('backend liveness endpoint responds from the composed application', async ({ request }) => {
    await expect
      .poll(
        async () => {
          const response = await request.get(`${apiBaseURL}/health`);
          return { status: response.status(), body: await response.text() };
        },
        { timeout: 60_000 },
      )
      .toEqual({ status: 200, body: 'ok' });
  });

  test('backend readiness endpoint reaches its dependencies', async ({ request }) => {
    await expect
      .poll(
        async () => {
          const response = await request.get(`${apiBaseURL}/health?ready=1`);
          return { status: response.status(), body: await response.text() };
        },
        { timeout: 60_000 },
      )
      .toEqual({ status: 200, body: 'ok' });
  });

  test('authenticated browser session survives a full page reload', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();

    await page.reload();

    await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
    await expect(page.getByText('ワークスペース', { exact: true }).first()).toBeVisible();
  });
});
