import { expect, test } from './fixtures/test';
import { chooseFile, createWorkspace } from './helpers/workspace';

const gcsURL = process.env.E2E_GCS_URL ?? 'http://127.0.0.1:4443';

async function listStoredObjects(prefix: string): Promise<string[]> {
  const res = await fetch(`${gcsURL}/storage/v1/b/synthify-uploads/o?prefix=${encodeURIComponent(prefix)}`);
  expect(res.ok).toBe(true);
  const body = (await res.json()) as { items?: { name: string }[] };
  return (body.items ?? []).map((item) => item.name);
}

test('deletes a processed document with its tree nodes and stored files', async ({ page }) => {
  test.setTimeout(90_000);
  await page.goto('/');
  await createWorkspace(page);

  const created = page.waitForResponse('**/synthify.app.v1.DocumentService/CreateDocument');
  await chooseFile(page, { name: 'e2e-delete.md', mimeType: 'text/markdown', buffer: Buffer.from('# delete me') });
  const { document } = (await (await created).json()) as { document: { documentId: string; workspaceId: string } };
  const documentPrefix = `${document.workspaceId}/${document.documentId}/`;

  const report = page.frameLocator('iframe[title="workspace-root-content"]').getByRole('heading', { name: 'E2E Fixture Report' });
  await expect(report).toBeVisible({ timeout: 30_000 });
  expect(await listStoredObjects(documentPrefix)).not.toHaveLength(0);

  const list = page.getByTestId('workspace-document-list').last();
  await expect(list.getByText('e2e-delete.md', { exact: true })).toBeVisible();
  await list.getByRole('button', { name: 'e2e-delete.md を削除' }).click();
  await expect(list.getByText(/元に戻せません/)).toBeVisible();
  await list.getByRole('button', { name: '削除する' }).click();

  await expect(page.getByTestId('workspace-document-list')).toHaveCount(0);
  await expect(report).toHaveCount(0);
  expect(await listStoredObjects(documentPrefix)).toEqual([]);

  await page.reload();
  await expect(page.getByText('ファイルをアップロード').last()).toBeVisible();
  await expect(page.getByText('e2e-delete.md', { exact: true })).toHaveCount(0);
});
