import { expect, test } from './fixtures/test';

// Demo capture for the STIF import UI. This is not part of the test suite (the
// filename ends in .demo.ts, not .spec.ts, so testDir's default match skips
// it); run it explicitly with a video-recording config to produce the clip
// attached to the PR:
//
//   npx playwright test e2e/stif-import.demo.ts --config=playwright.demo.config.ts
//
// It is written as a demo, not an assertion suite: the pauses exist so the
// recording is watchable, and the document is a realistic one rather than the
// minimum that parses.

// The documents are deliberately ASCII-only. The textarea renders in a
// monospace face, and the browser this records in has no CJK glyphs for it, so
// Japanese in the source would come out as tofu boxes in the clip.
const STIF_DOCUMENT = `#STIF/1
@tree mode="append" scope="canonical"

::item /overview
title: "Synthify Architecture"
description: "How the pieces fit together"
order: 10
---
<article>
  <h1>Synthify Architecture</h1>
  <p>Three layers and the contract between them.</p>
</article>
::css
article { line-height: 1.7; }
::end

::item /overview/api
title: "API Layer"
description: "Go service exposing Connect RPC"
parent: /overview
order: 10
source:
  - type: github
    repo: Keyhole-Koro/Synthify
    ref: main
    path: apps/api/internal/handler
    confidence: 0.9
---
<p>handler translates proto to domain, nothing more.</p>
::end

::item /overview/worker
title: "Worker Layer"
description: "LLM tool execution and job processing"
parent: /overview
order: 20
---
<p>Builds the knowledge tree; progress flows through Firestore.</p>
::end

::item /overview/web
title: "Web Layer"
description: "Single-page paper-in-paper UI"
parent: /overview
order: 30
---
<p>The workspace paper is the entry point to the tree.</p>
::end
`;

// A document with two deliberate mistakes, to show the diagnostics path:
// /broken/child points at a parent that does not exist, and /broken carries a
// script tag the sanitizer strips.
const STIF_WITH_PROBLEMS = `#STIF/1

::item /broken
title: "Has a problem"
---
<p>Body survives</p><script>steal()</script>
::end

::item /broken/child
title: "Orphan"
parent: /does-not-exist
---
<p>No parent</p>
::end
`;

const beat = (page: import('@playwright/test').Page, ms = 1200) => page.waitForTimeout(ms);

// The workspace paper sits in a small room in the paper-in-paper layout, and
// the import panel is taller than that room. Keeping the element of the moment
// scrolled into view is what makes the clip show the feature rather than a
// cropped textarea.
async function focusOn(locator: import('@playwright/test').Locator) {
  await locator.scrollIntoViewIfNeeded();
}

test('STIF import: preview, diagnostics, then commit', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();

  // Open a workspace to import into.
  await page.getByText('ワークスペース', { exact: true }).first().click();
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();
  const workspaceHeading = page.getByRole('button', { name: '新規ワークスペース', exact: true }).last();
  await expect(workspaceHeading).toBeVisible();

  // Close the sibling papers so the workspace paper gets the room; otherwise
  // the import panel is a letterbox in the corner.
  for (const name of ['アカウント', 'Synthify について']) {
    const closeButton = page.locator(`[data-paper-id]`).filter({ hasText: name }).getByRole('button', { name: '閉じる' });
    if (await closeButton.count()) await closeButton.first().click().catch(() => {});
  }
  await beat(page);

  // Open the import panel.
  await page.getByRole('button', { name: 'インポート' }).last().click();
  const textarea = page.getByLabel('STIF ドキュメント');
  await expect(textarea).toBeVisible();
  await focusOn(textarea);
  await beat(page);

  // 1. A document with problems: the preview refuses it and says why.
  await textarea.fill(STIF_WITH_PROBLEMS);
  await beat(page, 800);
  await page.getByRole('button', { name: 'プレビュー' }).click();
  const unresolvedParent = page.getByText(/parent .* is not defined/);
  await expect(unresolvedParent).toBeVisible();
  await expect(page.getByText(/エラーがあるためインポートできません/)).toBeVisible();
  await focusOn(unresolvedParent);
  await beat(page, 3000);

  // 2. The real document: preview shows the tree it would build.
  await textarea.fill(STIF_DOCUMENT);
  await beat(page, 800);
  await page.getByRole('button', { name: 'プレビュー' }).click();
  // Scope to the preview list: the titles also appear in the textarea, which
  // Playwright's text matching sees.
  const previewList = page.getByTestId('stif-preview-items');
  await expect(previewList).toBeVisible();
  for (const title of ['Synthify Architecture', 'API Layer', 'Worker Layer', 'Web Layer']) {
    await expect(previewList.getByText(title, { exact: true })).toBeVisible();
  }
  await focusOn(previewList);
  await beat(page, 3500);

  // 3. Commit.
  await page.getByRole('button', { name: /件をインポート/ }).click();
  const done = page.getByText(/件のアイテムをツリーに追加しました/);
  await expect(done).toBeVisible();
  await focusOn(done);
  await beat(page, 3000);
});
