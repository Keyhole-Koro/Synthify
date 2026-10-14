import { expect, test } from './fixtures/test';

test.use({ storageState: { cookies: [], origins: [] } });

test('shows the login entry point without a session', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Google でログイン' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'ログアウト' })).toHaveCount(0);
});
