import { expect, test } from './fixtures/test';
import type { Page } from '@playwright/test';

// このファイルは常に動画を録る。チャットは「入力 → 回答 → 出典」までの
// 一連の流れを見ないと壊れ方が分からないので、失敗時だけの録画では足りない。
test.use({ video: 'on' });

test.describe.configure({ mode: 'serial' });

// ローカルの e2e スタックは実行のたびに workspace が積み上がり、一覧の
// 描画が目に見えて遅くなる。既定の 10s だと落ちることがあるので、一覧を
// 開く操作にだけ長めの待ちを与える。
const WORKSPACE_LIST_TIMEOUT = 30_000;

async function openWorkspaceList(page: Page) {
  const opener = page.getByText('ワークスペース', { exact: true }).first();
  await expect(opener).toBeVisible({ timeout: WORKSPACE_LIST_TIMEOUT });
  await opener.click();
}

async function openSeededWorkspace(page: Page) {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible({
    timeout: WORKSPACE_LIST_TIMEOUT,
  });

  await openWorkspaceList(page);
  await expect(page.getByRole('button', { name: '新規ワークスペース' }).first()).toBeVisible({
    timeout: WORKSPACE_LIST_TIMEOUT,
  });

  // seed は処理済みドキュメント (chunk + succeeded job) を1件作る。
  // これが出典付き回答 (grounded=true) の前提になる。
  await page.getByRole('button', { name: 'Dev seed workspace' }).click();

  // seed 直後の workspace は一覧の in-memory state に入らないことがあるので、
  // 一度リロードして永続化された一覧から開く (workspace.spec.ts と同じ手順)。
  await page.reload();
  await openWorkspaceList(page);
  const seeded = page.getByText('Synthify Dev Seed', { exact: true }).first();
  await expect(seeded).toBeVisible({ timeout: WORKSPACE_LIST_TIMEOUT });
  await seeded.click();

  await expect(page.getByTestId('workspace-chat')).toBeVisible({
    timeout: WORKSPACE_LIST_TIMEOUT,
  });
}

test('answers a question about the workspace and cites its sources', async ({ page }) => {
  await openSeededWorkspace(page);

  const input = page.getByTestId('workspace-chat-input');
  await expect(input).toBeEnabled();

  await input.fill('ワークスペースの権限について教えて');
  await page.getByTestId('workspace-chat-send').click();

  // 質問は送信直後に永続化され、そのまま履歴に残る。
  await expect(page.getByTestId('workspace-chat-message-user')).toContainText(
    'ワークスペースの権限について教えて',
  );

  const answer = page.getByTestId('workspace-chat-message-assistant');
  await expect(answer).toBeVisible();

  // 出典は必ずサーバーが検証した候補から出る。retrieval が質問に対応する
  // 節を最上位に並べるので、権限の質問なら権限の節が最初に来る。
  const sources = page.getByTestId('workspace-chat-sources').first();
  await expect(sources).toBeVisible();
  await expect(sources.locator('li').first()).toContainText('ワークスペースと権限');

  // 送信後は入力が空に戻り、続けて質問できる。
  await expect(input).toHaveValue('');
});

test('keeps the conversation across a reload', async ({ page }) => {
  await openSeededWorkspace(page);

  const input = page.getByTestId('workspace-chat-input');
  await input.fill('結論は何ですか');
  await page.getByTestId('workspace-chat-send').click();
  await expect(page.getByTestId('workspace-chat-message-assistant').last()).toBeVisible();

  await page.reload();
  await openWorkspaceList(page);
  await page.getByText('Synthify Dev Seed', { exact: true }).first().click();

  // 会話はサーバーに保存されているので、リロード後も履歴が戻る。
  await expect(page.getByTestId('workspace-chat-message-user').filter({ hasText: '結論は何ですか' })).toBeVisible();
});

test('asks follow-up questions in the same conversation', async ({ page }) => {
  await openSeededWorkspace(page);

  const input = page.getByTestId('workspace-chat-input');
  const send = page.getByTestId('workspace-chat-send');
  const answers = page.getByTestId('workspace-chat-message-assistant');
  const questions = page.getByTestId('workspace-chat-message-user');

  // 入力欄は履歴の復元が終わってから描画される。これを待たずに数えると
  // 復元前の 0 件を基準にしてしまう。
  await expect(input).toBeEnabled();

  // seed workspace はテスト間で使い回され、会話も復元されるので、件数は
  // 絶対値ではなく開始時点からの増分で見る。
  const answersBefore = await answers.count();
  const questionsBefore = await questions.count();

  await input.fill('ナレッジツリーとは何ですか');
  await send.click();
  await expect(answers).toHaveCount(answersBefore + 1);

  await input.fill('処理パイプラインの流れは');
  await send.click();
  await expect(answers).toHaveCount(answersBefore + 2);

  // 2 往復ぶんが同じ会話に積まれる。
  await expect(questions).toHaveCount(questionsBefore + 2);
});

// 資料が無くても質問できる。回答は「資料に基づかない」と明示される。
test('answers in a workspace with no documents, marked as ungrounded', async ({ page }) => {
  await page.goto('/');
  await openWorkspaceList(page);
  await page.getByRole('button', { name: '新規ワークスペース' }).first().click();

  const input = page.getByTestId('workspace-chat-input');
  await expect(input).toBeEnabled({ timeout: WORKSPACE_LIST_TIMEOUT });
  await expect(page.getByTestId('workspace-chat-hint')).toContainText('まだ資料もページもありません');

  await input.fill('このワークスペースは何に使えますか');
  await page.getByTestId('workspace-chat-send').click();

  await expect(page.getByTestId('workspace-chat-message-assistant').last()).toBeVisible();
  await expect(page.getByTestId('workspace-chat-ungrounded').last()).toBeVisible();
  await expect(page.getByTestId('workspace-chat-sources')).toHaveCount(0);
});
