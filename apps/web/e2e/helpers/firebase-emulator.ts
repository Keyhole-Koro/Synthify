import type { Page } from '@playwright/test';

const authEmulatorURL = process.env.E2E_FIREBASE_AUTH_URL ?? 'http://127.0.0.1:9099';
const apiKey = 'demo-api-key';

export type TestUser = { email: string; password: string };

export async function createTestUser(): Promise<TestUser> {
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const user = { email: `e2e-${suffix}@example.test`, password: `E2e-${suffix}!` };
  
  let response: Response | undefined;
  for (let i = 0; i < 10; i++) {
    try {
      response = await fetch(
        `${authEmulatorURL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=${apiKey}`,
        {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ ...user, returnSecureToken: true }),
        },
      );
      break;
    } catch (error) {
      if (i === 9) throw error;
      await new Promise(r => setTimeout(r, 1000));
    }
  }

  if (!response?.ok) {
    const errorText = response ? await response.text() : 'No response';
    throw new Error(`Auth Emulator user creation failed: ${response?.status} ${errorText}`);
  }
  return user;
}

export async function signInThroughApp(page: Page, user: TestUser): Promise<void> {
  await page.goto('/');
  await page.waitForFunction(() => '__synthifyE2E' in window);
  await page.evaluate(async ({ email, password }) => {
    const hook = (window as Window & {
      __synthifyE2E: { signIn(email: string, password: string): Promise<void> };
    }).__synthifyE2E;
    await hook.signIn(email, password);
  }, user);
}
