import { expect, test } from './fixtures/test';

test.describe.configure({ mode: 'serial' });

async function openWorkspacePanel(page: import('@playwright/test').Page) {
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await expect(page.getByRole('button', { name: '新規ワークスペース' }).first()).toBeVisible();
}

test('creates a workspace and restores it after reload', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  await openWorkspacePanel(page);

  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  const workspace = page.getByText('新規ワークスペース', { exact: true }).first();
  await expect(workspace).toBeVisible();

  await page.reload();
  await expect(page.getByText('新規ワークスペース', { exact: true }).first()).toBeVisible();
});

test('renames a workspace and keeps the new name after reload', async ({ page }) => {
  const renamed = `E2E Rename ${Date.now()}`;
  await page.goto('/');
  await openWorkspacePanel(page);
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();

  const workspaceHeading = page.getByRole('button', { name: '新規ワークスペース', exact: true }).last();
  await expect(workspaceHeading).toBeVisible();
  await workspaceHeading.click({ force: true });

  const nameInput = page.getByRole('textbox').last();
  await expect(nameInput).toBeVisible();
  await nameInput.fill(renamed);
  await nameInput.press('Enter');
  await expect(page.getByRole('button', { name: renamed, exact: true }).last()).toBeVisible();

  await page.reload();
  await openWorkspacePanel(page);
  await expect(page.getByText(renamed, { exact: true }).first()).toBeVisible();
  await page.getByText(renamed, { exact: true }).first().click();
  const renamedWorkspaceHeading = page.getByRole('button', { name: renamed, exact: true }).last();
  await renamedWorkspaceHeading.click({ force: true });
  const cancelNameInput = page.getByRole('textbox').last();
  await cancelNameInput.fill('   ');
  await cancelNameInput.press('Enter');
  await expect(page.getByText('ワークスペース名を入力してください。', { exact: true })).toBeVisible();
  await cancelNameInput.fill('This draft must not be saved');
  await page.getByRole('button', { name: '取消' }).last().click();

  await expect(page.getByRole('button', { name: renamed, exact: true }).last()).toBeVisible();
  await expect(page.getByText('This draft must not be saved', { exact: true })).toHaveCount(0);
  await page.reload();
  await openWorkspacePanel(page);
  await expect(page.getByText(renamed, { exact: true }).first()).toBeVisible();
  await expect(page.getByText('This draft must not be saved', { exact: true })).toHaveCount(0);
});

test('deletes only the selected workspace and does not restore it after reload', async ({ page }) => {
  await page.goto('/');
  await openWorkspacePanel(page);
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await expect(page.getByText('新規ワークスペース', { exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: 'Dev seed workspace' }).click();
  await page.reload();
  await openWorkspacePanel(page);
  await page.getByText('Synthify Dev Seed', { exact: true }).first().click();
  await expect(page.getByRole('button', { name: '削除', exact: true }).last()).toBeVisible();

  await page.getByRole('button', { name: '削除', exact: true }).last().click();
  await expect(page.getByRole('button', { name: '削除を確定' }).last()).toBeVisible();
  await page.getByRole('button', { name: 'キャンセル', exact: true }).last().click();
  await expect(page.getByText('Synthify Dev Seed', { exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: '削除', exact: true }).last().click();
  await page.getByRole('button', { name: '削除を確定' }).last().click();
  await expect(page.getByText('Synthify Dev Seed', { exact: true })).toHaveCount(0);
  await expect(page.getByText('新規ワークスペース', { exact: true }).first()).toBeVisible();

  await page.reload();
  await openWorkspacePanel(page);
  await expect(page.getByText('Synthify Dev Seed', { exact: true })).toHaveCount(0);
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
