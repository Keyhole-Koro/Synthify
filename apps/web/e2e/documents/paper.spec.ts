import { expect, test } from './fixtures/test';

async function uploadFixture(page: import('@playwright/test').Page) {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  const [chooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.getByText('ファイルをアップロード').last().click(),
  ]);
  await chooser.setFiles({ name: `paper-${Date.now()}.md`, mimeType: 'text/markdown', buffer: Buffer.from('# fixture') });
  await expect(page.frameLocator('iframe[title="workspace-root-content"]').getByRole('heading', { name: 'E2E Fixture Report' })).toBeVisible({ timeout: 30_000 });
}

test('opens and closes nested papers, renders generated HTML, and restores open state', async ({ page }) => {
  test.setTimeout(90_000);
  await uploadFixture(page);

  const rootFrame = page.frameLocator('iframe[title="workspace-root-content"]');
  await expect(rootFrame.getByRole('heading', { name: 'E2E Fixture Report' })).toBeVisible();
  await rootFrame.getByText('E2E Fixture Overview', { exact: true }).click();
  await expect(page.getByRole('button', { name: 'E2E Fixture Overview' })).toBeVisible();
  const detailLink = page.locator('iframe[title^="paper-content-"]').first().contentFrame().getByText('E2E Fixture Detail', { exact: true });
  await expect(detailLink).toBeVisible();
  await detailLink.click();
  await expect(page.getByRole('button', { name: 'E2E Fixture Detail' })).toBeVisible();
  const evidenceLink = page.locator('iframe[title^="paper-content-"]').last().contentFrame().getByText('E2E Fixture Evidence', { exact: true });
  await expect(evidenceLink).toBeVisible();
  await evidenceLink.click();
  await expect(page.getByRole('button', { name: 'E2E Fixture Evidence' })).toBeVisible();

  await page.reload();
  await expect(page.getByRole('button', { name: 'E2E Fixture Detail' }).first()).toBeVisible();
  await expect(page.getByRole('button', { name: 'E2E Fixture Evidence' }).first()).toBeVisible();

  const close = page.getByRole('button', { name: /閉じる|Close/ }).last();
  if (await close.count()) await close.click();
});
