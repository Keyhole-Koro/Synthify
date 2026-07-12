import { expect, test } from './fixtures/test';
import { createTestUser, signInThroughApp } from './helpers/firebase-emulator';

test('owner invites editor/viewer, changes role, and removes the member', async ({ browser, page }) => {
  test.setTimeout(120_000);
  const member = await createTestUser();
  const context = await browser.newContext();
  const provisionPage = await context.newPage();
  await Promise.all([
    provisionPage.waitForResponse('**/synthify.app.v1.UserService/SignInUser'),
    signInThroughApp(provisionPage, member),
  ]);
  await expect(provisionPage.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  await context.close();
  await page.goto('/');
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await page.getByRole('button', { name: '共有', exact: true }).last().click();
  await page.getByLabel('招待メールアドレス').fill(member.email);
  await page.getByLabel('メンバー権限').selectOption('1');
  await page.getByRole('button', { name: '招待', exact: true }).click();
  await expect(page.getByText(member.email, { exact: true })).toBeVisible();
  const role = page.getByLabel(`${member.email} の権限`);
  await expect(role).toHaveValue('1');
  await role.selectOption('2');
  await expect(role).toHaveValue('2');
  const [removeResponse] = await Promise.all([
    page.waitForResponse('**/synthify.app.v1.WorkspaceService/RemoveMember'),
    page.getByRole('listitem').filter({ hasText: member.email }).getByRole('button', { name: '削除' }).click(),
  ]);
  expect(removeResponse.ok()).toBeTruthy();
  await page.reload();
  await expect(page.getByText(member.email, { exact: true })).toHaveCount(0);
});
