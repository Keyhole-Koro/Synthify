import { expect, test } from './fixtures/test';
import { createTestUser, signInThroughApp } from './helpers/firebase-emulator';

test('restores the authenticated session after reload', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Google でログイン' })).toHaveCount(0);
});

test('hides authenticated workspace data after logout and reload', async ({ page }) => {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).click();
  await expect(page.getByText('新規ワークスペース', { exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: 'ログアウト' }).click();
  await expect(page.getByRole('button', { name: 'Google でログイン' })).toBeVisible();
  await expect(page.getByText('新規ワークスペース', { exact: true })).toHaveCount(0);

  await page.reload();
  await expect(page.getByRole('button', { name: 'Google でログイン' })).toBeVisible();
  await expect(page.getByText('新規ワークスペース', { exact: true })).toHaveCount(0);
});

test('does not expose one user workspace cache or open papers to another user', async ({ page }) => {
  const privateName = 'Synthify Dev Seed';
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: 'Dev seed workspace' }).click();
  await page.reload();
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await expect(page.getByText(privateName, { exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: 'ログアウト' }).click();
  const secondUser = await createTestUser();
  await signInThroughApp(page, secondUser);
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  await expect(page.getByText(privateName, { exact: true })).toHaveCount(0);

  await page.reload();
  await expect(page.getByText(privateName, { exact: true })).toHaveCount(0);
});
