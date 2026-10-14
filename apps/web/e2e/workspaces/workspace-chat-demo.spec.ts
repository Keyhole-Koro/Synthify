import { expect, test } from './fixtures/test';
import type { Page } from '@playwright/test';

// レビュー用のデモ録画。挙動を確認するのが目的で、検証は
// workspace-chat.spec.ts が持つ。@demo タグで PR 用の suite から外す。
//
// 録画を人が見て分かる速さにするため、意図的に間を置く。通常の e2e では
// waitForTimeout を使わない方針だが、このファイルは「読める録画を作る」ことが
// 目的なので例外とする。
test.use({
  video: { mode: 'on', size: { width: 1280, height: 800 } },
  viewport: { width: 1280, height: 800 },
});

// 間を置きながら4つの経路を通すので、既定の 60s には収まらない。
test.setTimeout(180_000);

const BEAT = 900;

async function openSeededWorkspaceAgain(page: Page) {
  await page.goto('/');
  const list = page.getByText('ワークスペース', { exact: true }).first();
  await expect(list).toBeVisible({ timeout: 30_000 });
  await list.click();
  await page.getByText('Synthify Dev Seed', { exact: true }).first().click();
  await expect(page.getByTestId('workspace-chat')).toBeVisible({ timeout: 30_000 });
}

async function beat(page: Page, factor = 1) {
  await page.waitForTimeout(BEAT * factor);
}

test('@demo workspace chat walkthrough', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible({ timeout: 30_000 });
  await beat(page);

  const opener = page.getByText('ワークスペース', { exact: true }).first();
  await expect(opener).toBeVisible({ timeout: 30_000 });
  await opener.click();
  await expect(page.getByRole('button', { name: '新規ワークスペース' }).first()).toBeVisible({
    timeout: 30_000,
  });
  await beat(page);

  await page.getByRole('button', { name: 'Dev seed workspace' }).click();
  await page.reload();

  const reopened = page.getByText('ワークスペース', { exact: true }).first();
  await expect(reopened).toBeVisible({ timeout: 30_000 });
  await reopened.click();

  const seeded = page.getByText('Synthify Dev Seed', { exact: true }).first();
  await expect(seeded).toBeVisible({ timeout: 30_000 });
  await seeded.click();

  const chat = page.getByTestId('workspace-chat');
  await expect(chat).toBeVisible({ timeout: 30_000 });
  await chat.scrollIntoViewIfNeeded();
  await beat(page, 1.5);

  const input = page.getByTestId('workspace-chat-input');
  const send = page.getByTestId('workspace-chat-send');
  await expect(input).toBeEnabled();

  // 1問目: 権限について尋ねる。retrieval が「ワークスペースと権限」を
  // 最上位に並べ、その節が出典として出る。
  await input.click();
  await input.pressSequentially('ワークスペースの権限について教えて', { delay: 55 });
  await beat(page);
  await send.click();

  await expect(page.getByTestId('workspace-chat-message-user').last()).toBeVisible();
  await expect(page.getByTestId('workspace-chat-message-assistant').last()).toBeVisible();

  const sources = page.getByTestId('workspace-chat-sources').last();
  await expect(sources).toBeVisible();
  await expect(sources.locator('li').first()).toContainText('ワークスペースと権限');
  await sources.scrollIntoViewIfNeeded();
  await beat(page, 2.5);

  // 2問目: 同じ会話に続けて質問する。別の節が最上位に来る。
  await input.click();
  await input.pressSequentially('処理パイプラインの流れは', { delay: 55 });
  await beat(page);
  await send.click();

  await expect(page.getByTestId('workspace-chat-message-assistant')).toHaveCount(2);
  const secondSources = page.getByTestId('workspace-chat-sources').last();
  await expect(secondSources.locator('li').first()).toContainText('処理パイプライン');
  await secondSources.scrollIntoViewIfNeeded();
  await beat(page, 2.5);

  // 資料が1件も無い workspace でも質問できる。回答には「資料に基づかない」
  // 旨が添えられる。
  await page.goto('/');
  const listAgain = page.getByText('ワークスペース', { exact: true }).first();
  await expect(listAgain).toBeVisible({ timeout: 30_000 });
  await listAgain.click();
  await beat(page);
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();

  const emptyInput = page.getByTestId('workspace-chat-input');
  await expect(emptyInput).toBeEnabled({ timeout: 30_000 });
  await expect(page.getByTestId('workspace-chat-hint')).toContainText('まだ資料もページもありません');
  await beat(page, 1.5);

  await emptyInput.click();
  await emptyInput.pressSequentially('このワークスペースは何に使えますか', { delay: 55 });
  await beat(page);
  await page.getByTestId('workspace-chat-send').click();

  await expect(page.getByTestId('workspace-chat-message-assistant').last()).toBeVisible();
  const ungrounded = page.getByTestId('workspace-chat-ungrounded').last();
  await expect(ungrounded).toBeVisible();
  // 回答とバッジはパネル下端に出るため、明示的に画面内へ送らないと録画に写らない。
  await ungrounded.scrollIntoViewIfNeeded();
  await beat(page, 3);

  // seed workspace に戻ると、さきほどの会話がサーバーから復元される。
  await openSeededWorkspaceAgain(page);

  await expect(
    page.getByTestId('workspace-chat-message-user').filter({ hasText: 'ワークスペースの権限について教えて' }),
  ).toBeVisible({ timeout: 30_000 });
  await page.getByTestId('workspace-chat').scrollIntoViewIfNeeded();
  await beat(page, 3);
});
