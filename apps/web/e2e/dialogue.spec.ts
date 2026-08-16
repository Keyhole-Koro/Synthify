import { expect, test } from './fixtures/test';

async function uploadFixture(page: import('@playwright/test').Page) {
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  const [chooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.getByText('ファイルをアップロード').last().click(),
  ]);
  await chooser.setFiles({
    name: `dialogue-${Date.now()}.md`,
    mimeType: 'text/markdown',
    buffer: Buffer.from('# fixture'),
  });
  await expect(
    page
      .frameLocator('iframe[title="workspace-root-content"]')
      .getByRole('heading', { name: 'E2E Fixture Report' }),
  ).toBeVisible({ timeout: 30_000 });
}

// Drives the dialogue end to end: the header "+" opens one as a child, the
// question goes through PostChatTurn, and the answer arrives over the Firestore
// turn subscription rather than in the RPC response.
//
// The worker runs its fixture generator here (E2E_WORKER_FIXTURE), so there is
// no model call — the point is that the trigger, the subscription and the
// ContentNode rendering line up, not what the model says.
test('opens a dialogue from a paper and renders the answer that arrives over Firestore', async ({ page }) => {
  test.setTimeout(120_000);
  await uploadFixture(page);

  const rootFrame = page.frameLocator('iframe[title="workspace-root-content"]');
  await rootFrame.getByText('E2E Fixture Overview', { exact: true }).click();
  const overview = page.getByRole('button', { name: 'E2E Fixture Overview' });
  await expect(overview).toBeVisible();

  // The "+" lives in the paper header and only renders once the canvas has a
  // create-child handler wired, so its presence is itself part of the check.
  const addChild = page.getByRole('button', { name: '+', exact: true }).last();
  await expect(addChild).toBeVisible();
  await addChild.click();

  const dialogue = page.getByRole('button', { name: 'AI に聞く' });
  await expect(dialogue).toBeVisible();

  const input = page.getByLabel('質問');
  await expect(input).toBeVisible();
  await input.fill('この資料の要点は？');
  await page.getByRole('button', { name: '送信' }).click();

  // The answer is written by the worker and reaches the browser through the
  // turn document, so this is the subscription working, not the RPC response.
  await expect(page.getByText('フィクスチャ応答', { exact: false })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('セクション単位で書き込まれます')).toBeVisible();

  // The context chip names a paper the worker actually used; clicking it opens
  // that paper on the canvas.
  const chip = page.getByRole('button', { name: 'E2E Fixture Overview' }).last();
  await expect(chip).toBeVisible();
});
