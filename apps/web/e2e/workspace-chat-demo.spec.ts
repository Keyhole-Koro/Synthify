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

const BEAT = 900;

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
  await chat.scrollIntoViewIfNeeded();
  await beat(page, 2.5);

  // 2問目: 同じ会話に続けて質問する。別の節が最上位に来る。
  await input.click();
  await input.pressSequentially('処理パイプラインの流れは', { delay: 55 });
  await beat(page);
  await send.click();

  await expect(page.getByTestId('workspace-chat-message-assistant')).toHaveCount(2);
  await expect(page.getByTestId('workspace-chat-sources').last().locator('li').first()).toContainText(
    '処理パイプライン',
  );
  await chat.scrollIntoViewIfNeeded();
  await beat(page, 2.5);

  // リロードしても会話はサーバーから戻る。
  await page.reload();
  const afterReload = page.getByText('ワークスペース', { exact: true }).first();
  await expect(afterReload).toBeVisible({ timeout: 30_000 });
  await afterReload.click();
  await page.getByText('Synthify Dev Seed', { exact: true }).first().click();

  await expect(
    page.getByTestId('workspace-chat-message-user').filter({ hasText: 'ワークスペースの権限について教えて' }),
  ).toBeVisible({ timeout: 30_000 });
  await page.getByTestId('workspace-chat').scrollIntoViewIfNeeded();
  await beat(page, 3);
});
