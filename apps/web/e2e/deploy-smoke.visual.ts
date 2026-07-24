import { expect, test } from '@playwright/test';

// Post-deploy visual QA against a live environment (stage/prod). These specs
// run in the dedicated `visual` Playwright project, which points at
// E2E_BASE_URL and does NOT authenticate — Phase 1 covers only the
// unauthenticated surface. Authenticated screens (which need a seeded stage QA
// account) are Phase 2. Kept separate from the emulator-backed e2e specs so a
// broken deploy can fail loudly without depending on the local compose stack.

// The landing route mounts the paper-in-paper canvas, whose nodes animate into
// place. Wait for the unauthenticated login CTA to settle before asserting or
// snapshotting so we compare a stable frame, not a mid-transition one.
async function gotoSettledLanding(page: import('@playwright/test').Page) {
  await page.goto('/', { waitUntil: 'networkidle' });
  await expect(page.getByRole('button', { name: 'Google でログイン' })).toBeVisible();
  // Let the 0.32s frame transitions finish; `animations: 'disabled'` freezes
  // them for the snapshot, but the liveness assertions run before that hook.
  await page.waitForTimeout(500);
}

test.describe('deploy smoke (unauthenticated)', () => {
  // A — liveness: the deploy actually serves a working app, not a blank page
  // or a JS-crashed shell that curl-based smoke tests happily report as 200.
  test('@smoke landing renders without runtime errors', async ({ page }) => {
    const consoleErrors: string[] = [];
    const pageErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('pageerror', (err) => pageErrors.push(err.message));

    const response = await page.goto('/', { waitUntil: 'domcontentloaded' });
    expect(response?.ok(), `landing responded ${response?.status()}`).toBeTruthy();

    // Chrome (brand header) and the unauthenticated CTA must both mount.
    await expect(page.getByText('Synthify', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Google でログイン' })).toBeVisible();

    expect(pageErrors, 'uncaught page errors').toEqual([]);
    expect(consoleErrors, 'console errors').toEqual([]);
  });

  // B — visual regression: compare the settled landing against the committed
  // baseline. `animations: 'disabled'` (configured project-wide) freezes the
  // paper-in-paper frame transitions so the diff is deterministic. If a live
  // environment surfaces a genuinely non-deterministic region (e.g. a demo
  // document image that varies), mask it here, e.g.
  //   mask: [page.getByRole('img', { name: /demo/ })]
  // then re-capture the baseline. See docs/visual-qa.md.
  test('@visual landing matches baseline', async ({ page }) => {
    await gotoSettledLanding(page);
    await expect(page).toHaveScreenshot('landing.png', { fullPage: true });
  });
});
