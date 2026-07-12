import { expect, test } from './fixtures/test';

const account = {
  accountId: 'e2e-account', plan: 'free', storageUsedBytes: '1048576', storageQuotaBytes: '10485760',
  budgetLimit: '100', currentPeriodUsage: '25', billingStatus: 'free',
};

test.beforeEach(async ({ page }) => {
  await page.route('**/synthify.app.v1.BillingService/GetBillingAccount', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(account) }));
});

async function openBilling(page: import('@playwright/test').Page) {
  await page.goto('/');
  await page.getByText('プラン・課金', { exact: true }).first().click();
  await expect(page.getByText('Free', { exact: true }).first()).toBeVisible();
}

test('shows free limits and sends checkout and portal requests', async ({ page }) => {
  const requests: string[] = [];
  await page.route('**/synthify.app.v1.BillingService/CreateCheckoutSession', async (route) => {
    requests.push(`checkout:${route.request().postData()}`);
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ checkoutUrl: 'https://checkout.stripe.test/e2e' }) });
  });
  await page.route('**/synthify.app.v1.BillingService/CreatePortalSession', async (route) => {
    requests.push(`portal:${route.request().postData()}`);
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ portalUrl: 'https://billing.stripe.test/e2e' }) });
  });
  await openBilling(page);
  await expect(page.getByText('アップグレード', { exact: true }).first()).toBeVisible();
  await page.getByText('アップグレード', { exact: true }).first().click();
  await expect(page.getByRole('link', { name: /Stripe で決済/ })).toBeVisible();
  await page.getByText('管理', { exact: true }).first().click();
  await expect(page.getByRole('link', { name: /Stripe ポータル/ })).toBeVisible();
  expect(requests.some((item) => item.startsWith('checkout:'))).toBeTruthy();
  expect(requests.some((item) => item.startsWith('portal:'))).toBeTruthy();
});

test('updates budget and exposes billing API failures', async ({ page }) => {
  let budgetBody = '';
  await page.route('**/synthify.app.v1.BillingService/UpdateBudget', async (route) => {
    budgetBody = route.request().postData() ?? '';
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ budgetLimit: '75' }) });
  });
  await page.route('**/synthify.app.v1.BillingService/CreateCheckoutSession', (route) => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'unavailable', message: 'stripe unavailable' }) }));
  await openBilling(page);
  await page.getByText('予算設定', { exact: true }).first().click();
  const input = page.getByRole('spinbutton');
  await input.fill('75');
  await page.getByRole('button', { name: '保存', exact: true }).click();
  await expect(page.getByText('保存しました')).toBeVisible();
  expect(budgetBody).toContain('75');
  await page.getByText('アップグレード', { exact: true }).first().click();
  await expect(page.getByText('決済URLを取得できませんでした。')).toBeVisible();
});
