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

  // Two different ways the answer can end up unreadable, so both are checked.
  //
  // The first recording of this flow showed text cut off mid-sentence. The
  // cause was not wrapping — the answer wraps inside its own box — but the
  // dialogue paper being laid out partly past the canvas edge, so the viewport
  // clipped it.
  const long = page.getByText('長い文章が折り返されることを確認するための行です', { exact: false });

  const wrap = await long.evaluate((el) => {
    const box = el.closest('div');
    if (!box) return null;
    return { scrollOverflow: box.scrollWidth - box.clientWidth };
  });
  expect(wrap, 'answer container should be measurable').not.toBeNull();
  expect(wrap!.scrollOverflow, 'answer must wrap rather than overflow its container').toBeLessThanOrEqual(1);

  const onScreen = await long.evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { left: Math.round(r.left), right: Math.round(r.right), viewport: window.innerWidth };
  });
  expect(onScreen.left, 'answer must not be clipped by the left edge').toBeGreaterThanOrEqual(0);
  expect(onScreen.right, 'answer must not be clipped by the right edge').toBeLessThanOrEqual(onScreen.viewport);

  // A structural change is offered, not performed. It must be described in
  // words — an Apply button over raw JSON would be a blind click — and the
  // paper it would create must not exist until the reader asks for it.
  await expect(page.getByText('提案された変更')).toBeVisible();
  await expect(page.getByText('フィクスチャが提案する子 paper', { exact: false })).toBeVisible();
  await expect(page.getByRole('button', { name: 'フィクスチャが提案する子 paper' })).toHaveCount(0);

  await page.getByRole('button', { name: '適用', exact: true }).click();

  await expect(page.getByRole('button', { name: 'フィクスチャが提案する子 paper' })).toBeVisible();
  await expect(page.getByText('適用済み')).toBeVisible();
});
