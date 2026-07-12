import { expect, test } from './fixtures/test';

test.describe.configure({ mode: 'serial' });

async function chooseFile(page: import('@playwright/test').Page, file: { name: string; mimeType: string; buffer: Buffer }) {
  const [chooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.getByText('ファイルをアップロード').last().click(),
  ]);
  await chooser.setFiles(file);
}

test('uploads to fake GCS, restores running progress, and renders fixture tree after completion', async ({ page }) => {
  test.setTimeout(90_000);
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await chooseFile(page, { name: 'e2e-slow.md', mimeType: 'text/markdown', buffer: Buffer.from('# fixture') });

  await expect(page.getByText('解析中', { exact: true }).last()).toBeVisible();
  await page.reload();
  await expect(page.getByText(/解析中|Fixture processing/).last()).toBeVisible();
  const fixtureFrame = page.frameLocator('iframe[title="workspace-root-content"]');
  await expect(fixtureFrame.getByRole('heading', { name: 'E2E Fixture Report' })).toBeVisible({ timeout: 30_000 });
  await expect(fixtureFrame.getByText('E2E Fixture Overview', { exact: true })).toBeVisible();

  await page.reload();
  await expect(page.frameLocator('iframe[title="workspace-root-content"]').getByRole('heading', { name: 'E2E Fixture Report' })).toBeVisible();
});

test('rejects executable files without starting an upload', async ({ page }) => {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await expect(page.getByText('ファイルをアップロード').last()).toBeVisible();

  await chooseFile(page, {
    name: 'unsafe.exe',
    mimeType: 'application/x-msdownload',
    buffer: Buffer.from('not really executable'),
  });

  await expect(page.getByText('アップロードに失敗しました。実行ファイル形式はアップロードできません。', { exact: true })).toBeVisible();
  await expect(page.getByText('解析中', { exact: true })).toHaveCount(0);
});

test('surfaces an upload API failure and remains retryable', async ({ page }) => {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();

  await page.route('**/synthify.app.v1.DocumentService/CreateDocument', async (route) => {
    await route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ code: 'unavailable', message: 'E2E upload unavailable' }),
    });
  });
  await chooseFile(page, {
    name: 'retry.md',
    mimeType: 'text/markdown',
    buffer: Buffer.from('# Retry me'),
  });
  await expect(page.getByText(/アップロードに失敗しました|E2E upload unavailable/).first()).toBeVisible();

  await page.unroute('**/synthify.app.v1.DocumentService/CreateDocument');
  await expect(page.getByText('ファイルをアップロード').last()).toBeVisible();
  await expect(page.locator('input[type="file"]').last()).toBeEnabled();
});

test('shows a size limit error without uploading bytes', async ({ page }) => {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await page.route('**/synthify.app.v1.DocumentService/CreateDocument', async (route) => {
    await route.fulfill({ status: 413, contentType: 'application/json', body: JSON.stringify({ code: 'resource_exhausted', message: 'ファイルサイズが上限を超えています。' }) });
  });
  await chooseFile(page, { name: 'oversized.pdf', mimeType: 'application/pdf', buffer: Buffer.from('oversized') });
  await expect(page.getByText(/ファイルサイズが上限を超えています|アップロードに失敗しました/).first()).toBeVisible();
});

test('rejects upload when the free-plan quota is exhausted', async ({ page }) => {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await page.route('**/synthify.app.v1.DocumentService/CreateDocument', (route) => route.fulfill({
    status: 429,
    contentType: 'application/json',
    body: JSON.stringify({ code: 'resource_exhausted', message: '無料プランのストレージ上限に達しました。アップグレードしてください。' }),
  }));
  await chooseFile(page, { name: 'quota.md', mimeType: 'text/markdown', buffer: Buffer.from('# quota') });
  await expect(page.getByText(/ストレージ上限|アップグレード/).first()).toBeVisible();
  await expect(page.getByText('解析中', { exact: true })).toHaveCount(0);
});

test('surfaces StartProcessing failure after a successful fake GCS upload', async ({ page }) => {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await page.route('**/synthify.app.v1.DocumentService/StartProcessing', async (route) => {
    await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'unavailable', message: 'E2E processing unavailable' }) });
  });
  await chooseFile(page, { name: 'processing.md', mimeType: 'text/markdown', buffer: Buffer.from('# uploaded first') });
  await expect(page.getByText(/アップロードに失敗しました|E2E processing unavailable/).first()).toBeVisible();
});

test('shows worker failure and exposes retry upload control', async ({ page }) => {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await chooseFile(page, { name: 'e2e-fail.md', mimeType: 'text/markdown', buffer: Buffer.from('# fail fixture') });
  await expect(page.getByText('E2E fixture worker failure', { exact: true }).last()).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText('再試行', { exact: true }).last()).toBeVisible();
});
