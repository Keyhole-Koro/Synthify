import { expect, test } from '@playwright/test';

test('@nightly deployed frontend responds', async ({ page }) => {
  const response = await page.goto('/');
  expect(response?.ok()).toBeTruthy();
  await expect(page.locator('body')).not.toBeEmpty();
});
