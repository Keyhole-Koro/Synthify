import { expect, test } from './fixtures/test';
import { createTestUser, signInThroughApp } from './helpers/firebase-emulator';

// The local stack runs with intake open (SYNTHIFY_INTAKE_CLOSED unset). These
// tests make the page see it closed the two ways it can: SignInUser says so,
// or the API refuses a call the way it does while the setting is on.

// Users of their own: the workspaces these tests leave behind would crowd the
// shared e2e user's workspace list, which other specs work in.
test.use({ storageState: { cookies: [], origins: [] } });

const NOTICE = 'Synthify は新しい文書の受け付けと有料プランを終了しました。これまでに作ったツリーは、引き続き見る・共有する・削除することができます。';
const DOCUMENTS_CLOSED = '新しい文書の受け付けは終了しました。';

// synthify.app.v1.ErrorDetail{reason: ERROR_REASON_INTAKE_CLOSED}, as the API
// encodes it in a Connect error body.
const INTAKE_CLOSED_DETAIL = { type: 'synthify.app.v1.ErrorDetail', value: 'CAE' };

const freeAccount = {
  accountId: 'e2e-account', plan: 'free', storageUsedBytes: '1048576', storageQuotaBytes: '10485760',
  budgetLimit: '100', currentPeriodUsage: '25', billingStatus: 'free',
};

async function signInAsNewUser(page: import('@playwright/test').Page) {
  await signInThroughApp(page, await createTestUser());
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
}

async function closeIntakeInSignIn(page: import('@playwright/test').Page) {
  await page.route('**/synthify.app.v1.UserService/SignInUser', async (route) => {
    const response = await route.fetch();
    await route.fulfill({ response, json: { ...(await response.json()), intakeClosed: true } });
  });
}

test('closed intake shows the banner and leaves no way to upload or upgrade', async ({ page }) => {
  const checkouts: string[] = [];
  await page.route('**/synthify.app.v1.BillingService/GetBillingAccount', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(freeAccount) }));
  await page.route('**/synthify.app.v1.BillingService/CreateCheckoutSession', async (route) => {
    checkouts.push(route.request().url());
    await route.abort();
  });
  await closeIntakeInSignIn(page);

  await signInAsNewUser(page);
  await expect(page.getByText(NOTICE)).toBeVisible();
  await expect(page.getByText('ファイルをアップロード')).toHaveCount(0);

  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await expect(page.getByText(DOCUMENTS_CLOSED).last()).toBeVisible();
  await expect(page.getByText('ファイルをアップロード')).toHaveCount(0);
  // Sharing and deleting the workspace stay.
  await expect(page.getByRole('button', { name: '共有' }).last()).toBeVisible();
  await expect(page.getByRole('button', { name: '削除', exact: true }).last()).toBeVisible();

  await page.getByText('プラン・課金', { exact: true }).first().click();
  await expect(page.getByText('Stripe ポータル', { exact: true })).toBeVisible();
  await expect(page.getByText('アップグレード', { exact: true })).toHaveCount(0);
  expect(checkouts).toEqual([]);
});

test('a page loaded before intake closed shows the refusal and then hides uploads', async ({ page }) => {
  await page.route('**/synthify.app.v1.DocumentService/CreateDocument', (route) => route.fulfill({
    status: 400,
    contentType: 'application/json',
    body: JSON.stringify({ code: 'failed_precondition', message: DOCUMENTS_CLOSED, details: [INTAKE_CLOSED_DETAIL] }),
  }));

  await signInAsNewUser(page);
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await expect(page.getByRole('button', { name: '新規ワークスペース' }).first()).toBeVisible();
  // The first screen has an upload drop of its own: wait for the new
  // workspace's, which comes after it, before choosing the last one.
  const drops = page.getByText('ファイルをアップロード');
  const dropsBefore = await drops.count();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  await expect(drops).toHaveCount(dropsBefore + 1);
  await expect(page.getByText(NOTICE)).toHaveCount(0);

  const [chooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    drops.last().click(),
  ]);
  await chooser.setFiles({ name: 'late.md', mimeType: 'text/markdown', buffer: Buffer.from('# too late') });

  await expect(page.getByText(DOCUMENTS_CLOSED).last()).toBeVisible();
  await expect(page.getByText(NOTICE)).toBeVisible();
  await expect(page.getByText('ファイルをアップロード')).toHaveCount(0);
  await expect(page.getByText(/アップロードに失敗しました/)).toHaveCount(0);
});
