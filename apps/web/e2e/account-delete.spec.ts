import { expect, test } from './fixtures/test';
import { createTestUser, signInThroughApp } from './helpers/firebase-emulator';
import { chooseFile, createWorkspace } from './helpers/workspace';

const authEmulatorURL = process.env.E2E_FIREBASE_AUTH_URL ?? 'http://127.0.0.1:9099';
const gcsURL = process.env.E2E_GCS_URL ?? 'http://127.0.0.1:4443';

// A user of its own: deleting the shared e2e user would break the other specs.
test.use({ storageState: { cookies: [], origins: [] } });

test('deletes the account with its workspaces, stored files and sign-in', async ({ page }) => {
  test.setTimeout(120_000);
  const user = await createTestUser();
  await signInThroughApp(page, user);
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();

  await createWorkspace(page);
  const created = page.waitForResponse('**/synthify.app.v1.DocumentService/CreateDocument');
  await chooseFile(page, { name: 'e2e-account.md', mimeType: 'text/markdown', buffer: Buffer.from('# mine') });
  const { document } = (await (await created).json()) as { document: { workspaceId: string } };
  await expect(page.frameLocator('iframe[title="workspace-root-content"]').getByRole('heading', { name: 'E2E Fixture Report' }))
    .toBeVisible({ timeout: 30_000 });

  await page.getByRole('button', { name: 'アカウントを削除（退会）' }).click();
  await expect(page.getByRole('group', { name: 'アカウント削除の確認' })).toBeVisible();
  const deleted = page.waitForResponse('**/synthify.app.v1.UserService/DeleteAccount');
  await page.getByRole('button', { name: '退会する' }).click();
  expect((await deleted).ok()).toBe(true);
  await expect(page.getByRole('button', { name: 'ログアウト' })).toHaveCount(0);

  const signIn = await fetch(
    `${authEmulatorURL}/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=demo-api-key`,
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ email: user.email, password: user.password, returnSecureToken: true }),
    },
  );
  expect(signIn.ok, 'the sign-in user is deleted').toBe(false);

  const stored = await fetch(`${gcsURL}/storage/v1/b/synthify-uploads/o?prefix=${encodeURIComponent(`${document.workspaceId}/`)}`);
  expect(((await stored.json()) as { items?: unknown[] }).items ?? []).toEqual([]);
});
