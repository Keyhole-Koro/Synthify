import { test as base, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repoRoot = fileURLToPath(new URL('../../../../', import.meta.url));
const MAX_BODY_LENGTH = 4_000;

function redact(value: string): string {
  return value
    .replace(/(authorization["':=\s]+)(bearer\s+)?[^\s,"'}]+/gi, '$1[REDACTED]')
    .replace(/([?&](?:token|key)=)[^&\s]+/gi, '$1[REDACTED]')
    .replace(/("(?:idToken|refreshToken|accessToken)"\s*:\s*")[^"]+/gi, '$1[REDACTED]');
}

export const test = base.extend({
  page: async ({ page }, use, testInfo) => {
    const startedAt = new Date().toISOString();
    const browserEvents: string[] = [];
    const responseCaptures: Promise<void>[] = [];

    page.on('console', (message) => {
      browserEvents.push(`[console.${message.type()}] ${redact(message.text())}`);
    });
    page.on('pageerror', (error) => {
      browserEvents.push(`[pageerror] ${redact(error.stack ?? error.message)}`);
    });
    page.on('requestfailed', (request) => {
      browserEvents.push(`[requestfailed] ${request.method()} ${redact(request.url())} — ${request.failure()?.errorText ?? 'unknown'}`);
    });
    page.on('response', (response) => {
      if (response.status() < 400) return;
      responseCaptures.push((async () => {
        let body = '';
        try {
          const contentType = response.headers()['content-type'] ?? '';
          if (/json|text|connect/.test(contentType)) body = (await response.text()).slice(0, MAX_BODY_LENGTH);
        } catch {
          body = '[body unavailable]';
        }
        browserEvents.push(`[response] ${response.status()} ${response.request().method()} ${redact(response.url())}${body ? `\n${redact(body)}` : ''}`);
      })());
    });

    // Playwright names its fixture continuation `use`; this is not a React hook.
    // eslint-disable-next-line react-hooks/rules-of-hooks
    await use(page);
    await Promise.allSettled(responseCaptures);

    if (testInfo.status === testInfo.expectedStatus) return;

    await testInfo.attach('browser-diagnostics.txt', {
      body: Buffer.from(browserEvents.join('\n\n') || 'No browser errors captured.'),
      contentType: 'text/plain',
    });

    const compose = spawnSync(
      'docker',
      ['compose', 'logs', '--no-color', '--since', startedAt, 'frontend', 'backend', 'worker', 'firebase-auth'],
      { cwd: repoRoot, encoding: 'utf8', timeout: 15_000, maxBuffer: 2 * 1024 * 1024 },
    );
    const composeOutput = redact([compose.stdout, compose.stderr].filter(Boolean).join('\n'));
    await testInfo.attach('compose-logs.txt', {
      body: Buffer.from(composeOutput || `Compose logs unavailable (exit ${compose.status ?? 'unknown'}).`),
      contentType: 'text/plain',
    });
  },
});

export { expect };
