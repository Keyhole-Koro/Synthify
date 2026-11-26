import { expect, type Page } from '@playwright/test';

export type UploadFile = { name: string; mimeType: string; buffer: Buffer };

/**
 * Creates a workspace from the workspace list and waits for its own upload area.
 *
 * The home screen has an upload area too, earlier on the page, and a file chosen there goes to
 * another workspace that the home screen creates. Until the new workspace's area is there,
 * `chooseFile` (which takes the last one) would pick the home screen's, so whether a test
 * uploaded where it meant to depended on how fast the workspace opened.
 */
export async function createWorkspace(page: Page) {
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await expect(page.getByText('ファイルをアップロード')).toHaveCount(2);
}

/** Chooses a file in the last upload area on the page: the open workspace's after `createWorkspace`. */
export async function chooseFile(page: Page, file: UploadFile) {
  const [chooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.getByText('ファイルをアップロード').last().click(),
  ]);
  await chooser.setFiles(file);
}
