import { expect, test } from './fixtures/test';

test.use({ storageState: { cookies: [], origins: [] } });

test('links the terms and the privacy policy from sign-in and the footer', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Google でログイン' })).toBeVisible();

  const notice = page.locator('p', { hasText: 'に同意したものとみなします' });
  await expect(notice.getByRole('link', { name: '利用規約' })).toHaveAttribute('href', '/terms');
  await expect(notice.getByRole('link', { name: 'プライバシーポリシー' })).toHaveAttribute('href', '/privacy');

  const footer = page.getByRole('contentinfo');
  await expect(footer.getByRole('link', { name: '利用規約' })).toHaveAttribute('href', '/terms');
  await footer.getByRole('link', { name: 'プライバシーポリシー' }).click();

  await expect(page).toHaveURL(/\/privacy$/);
  await expect(page.getByRole('heading', { level: 1, name: 'プライバシーポリシー' })).toBeVisible();
});

test('shows both documents and moves between them', async ({ page }) => {
  await page.goto('/terms');
  await expect(page).toHaveTitle('利用規約 | Synthify');
  await expect(page.getByRole('heading', { level: 1, name: '利用規約' })).toBeVisible();
  await expect(page.getByRole('heading', { level: 2, name: '第1条（適用）' })).toBeVisible();

  await page.getByRole('contentinfo').getByRole('link', { name: 'プライバシーポリシー' }).click();
  await expect(page).toHaveURL(/\/privacy$/);
  await expect(page.getByRole('heading', { level: 2, name: /^第\d+条（お問い合わせ窓口）$/ })).toBeVisible();

  await page.getByRole('link', { name: 'Synthify' }).click();
  await expect(page.getByRole('button', { name: 'Google でログイン' })).toBeVisible();
});
