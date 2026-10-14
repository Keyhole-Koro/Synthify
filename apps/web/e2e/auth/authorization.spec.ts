import { expect, test } from './fixtures/test';
import { createTestUser, signInThroughApp } from './helpers/firebase-emulator';

test('owner invites editor/viewer, changes role, and removes the member', async ({ browser, page }) => {
  test.setTimeout(120_000);
  const member = await createTestUser();
  // storageState has to be cleared explicitly: inside a test, browser.newContext()
  // inherits the project's `use` options, so a bare newContext() opens already
  // signed in as the owner — including the Firebase session, which auth.setup
  // saves out of IndexedDB. That is what made this test flaky. The owner's page
  // load fired its own SignInUser, waitForResponse matched *that* one, and the
  // context was closed while the member's provisioning call was still in flight
  // (the API logged it as 499). InviteMember then found no user for the address
  // and failed with NotFound, which surfaced ten seconds later as "the invited
  // email never appeared".
  const context = await browser.newContext({ storageState: undefined });
  const provisionPage = await context.newPage();
  // Assert the provisioning call itself: the invite below can only succeed once
  // the member exists in our database, so a failure belongs here rather than in
  // a later expectation about the member list.
  const [provisioned] = await Promise.all([
    provisionPage.waitForResponse(
      (response) =>
        response.url().includes('/synthify.app.v1.UserService/SignInUser') &&
        response.request().method() === 'POST',
    ),
    signInThroughApp(provisionPage, member),
  ]);
  expect(provisioned.ok()).toBeTruthy();
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
