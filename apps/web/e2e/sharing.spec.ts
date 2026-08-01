import { expect, test } from './fixtures/test';

test.describe.configure({ mode: 'serial' });

test('creates a public read-only link and revokes access', async ({ browser, page }) => {
  test.setTimeout(120_000);
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: 'Dev seed workspace' }).click();
  await page.reload();
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByText('Synthify Dev Seed', { exact: true }).first().click();
  await page.getByRole('button', { name: '共有', exact: true }).last().click();

  // This test shares a workspace every other test also uses, and a link it
  // creates outlives a failed run. Start from a known state instead of
  // inheriting whatever earlier runs left behind: revoke every existing link
  // before creating the one under test. (.first() is deliberate here — the
  // point is to remove all of them, not to pick one.)
  const revokeButtons = page.getByTitle('リンクを失効');
  for (let remaining = await revokeButtons.count(); remaining > 0; remaining--) {
    await revokeButtons.first().click();
    await expect(revokeButtons).toHaveCount(remaining - 1);
  }

  await page.getByRole('button', { name: 'リンク作成' }).click();

  const linkCode = page.locator('code').last();
  await expect(linkCode).toContainText('/view?token=');
  const shareURL = (await linkCode.textContent())?.trim();
  expect(shareURL).toBeTruthy();

  const anonymous = await browser.newContext();
  const viewer = await anonymous.newPage();
  await viewer.goto(shareURL!);
  await expect(viewer.getByText('共有リンク · Read-only')).toBeVisible();
  await expect(viewer.getByRole('heading', { name: 'Synthify Dev Seed' })).toBeVisible();
  await expect(viewer.getByText('閲覧専用です。', { exact: false })).toBeVisible();
  await expect(viewer.getByRole('button', { name: '削除', exact: true })).toHaveCount(0);
  await expect(viewer.getByRole('button', { name: '共有', exact: true })).toHaveCount(0);

  // The list was emptied above and this test created exactly one link, so the
  // only revocable link is the one the viewer is holding. State that as an
  // assertion rather than leaving it implied: previously this was a bare
  // getByTitle().click(), which revoked "whichever link is first" and, once a
  // leftover existed, silently revoked an unrelated one — making the assertion
  // below fail for a reason that has nothing to do with sharing.
  await expect(revokeButtons).toHaveCount(1);
  await revokeButtons.click();
  await viewer.reload();
  await expect(viewer.getByRole('heading', { name: 'リンクを開けません' })).toBeVisible();
  await anonymous.close();
});

test('shows a terminal error for an invalid share token', async ({ browser }) => {
  const anonymous = await browser.newContext();
  const page = await anonymous.newPage();
  await page.goto('/view?token=not-a-valid-share-token');
  await expect(page.getByRole('heading', { name: 'リンクを開けません' })).toBeVisible();
  await expect(page.getByText('共有リンクを読み込んでいます…')).toHaveCount(0);
  await anonymous.close();
});
