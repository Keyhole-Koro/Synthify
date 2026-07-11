import { expect, test } from '@playwright/test';

test.describe.configure({ mode: 'serial' });

async function openWorkspacePanel(page: import('@playwright/test').Page) {
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await expect(page.getByRole('button', { name: '新規ワークスペース' })).toBeVisible();
}

test('creates a workspace and restores it after reload', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  await openWorkspacePanel(page);

  await page.getByRole('button', { name: '新規ワークスペース' }).click();
  const workspace = page.getByText('新規ワークスペース', { exact: true }).first();
  await expect(workspace).toBeVisible();

  await page.reload();
  await expect(page.getByText('新規ワークスペース', { exact: true }).first()).toBeVisible();
});

test('opens a deterministic seeded workspace', async ({ page }) => {
  await page.goto('/');
  await openWorkspacePanel(page);
  await page.getByRole('button', { name: 'Dev seed workspace' }).click();

  // Reload also exercises that the seed was persisted, rather than only
  // trusting the optimistic in-memory workspace state.
  await page.reload();
  await openWorkspacePanel(page);
  await page.getByText('Synthify Dev Seed', { exact: true }).first().click();
  await expect(page.getByText('5 docs', { exact: true })).toBeVisible();
});
