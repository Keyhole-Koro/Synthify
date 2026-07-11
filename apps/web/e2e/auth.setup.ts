import { expect, test as setup } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import { createTestUser, signInThroughApp } from './helpers/firebase-emulator';

const authFile = fileURLToPath(new URL('./.auth/user.json', import.meta.url));

setup('authenticate with Firebase Auth Emulator', async ({ page }) => {
  const user = await createTestUser();
  await signInThroughApp(page, user);
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  // Firebase Auth persists the session in IndexedDB, not cookies/localStorage.
  await page.context().storageState({ path: authFile, indexedDB: true });
});
