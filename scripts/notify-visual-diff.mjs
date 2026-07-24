#!/usr/bin/env node
// Post a Discord alert when post-deploy visual QA finds a diff, attaching the
// actual diff image(s) so QA can eyeball the regression without downloading the
// CI artifact. Mirrors the embed style used by readiness-monitor.yml, but adds
// multipart file attachments (which the reusable notify-discord.yml can't do).
//
// Invoked from the deploy-frontend `visual-qa` job on failure(). It is
// deliberately resilient: a missing webhook or no diff files is a no-op (the
// failing test is the real signal), but a webhook send error exits non-zero so
// a broken notifier is visible in the job log.
import { readdir, readFile, stat } from 'node:fs/promises';
import path from 'node:path';

const MAX_ATTACHMENTS = 4; // Discord allows 10; keep alerts skimmable.

const {
  DISCORD_ALERT_WEBHOOK_URL,
  ENVIRONMENT = 'unknown',
  FRONTEND_URL = '',
  GITHUB_SERVER_URL = 'https://github.com',
  GITHUB_REPOSITORY = '',
  GITHUB_RUN_ID = '',
  GITHUB_SHA = '',
  GITHUB_REF_NAME = '',
  GITHUB_ACTOR = '',
} = process.env;

const RESULTS_DIR = process.env.VISUAL_RESULTS_DIR ?? 'apps/web/test-results';

async function findDiffImages(dir) {
  const out = [];
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch {
    return out; // dir may not exist if the run crashed before writing results
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...(await findDiffImages(full)));
    } else if (entry.name.endsWith('-diff.png')) {
      out.push(full);
    }
  }
  return out;
}

async function main() {
  if (!DISCORD_ALERT_WEBHOOK_URL) {
    console.warn('notify-visual-diff: DISCORD_ALERT_WEBHOOK_URL not set, skipping.');
    return;
  }

  const diffs = (await findDiffImages(RESULTS_DIR)).slice(0, MAX_ATTACHMENTS);
  const runUrl = `${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}`;
  const shortSha = GITHUB_SHA.slice(0, 7);

  const description = [
    `**${ENVIRONMENT}**${FRONTEND_URL ? ` — ${FRONTEND_URL}` : ''}`,
    `Commit \`${shortSha || 'unknown'}\` on \`${GITHUB_REF_NAME}\` (by \`${GITHUB_ACTOR}\`)`,
    diffs.length
      ? `${diffs.length} screen(s) diverged from baseline. Diff image(s) attached.`
      : 'Visual QA failed but no diff image was produced — check the run log (likely a liveness/@smoke failure or a missing baseline).',
  ].join('\n');

  const embed = {
    title: `🖼️ Visual QA regression: ${ENVIRONMENT}`,
    description,
    url: runUrl,
    color: 15158332, // red, matching the other alert embeds
  };

  const form = new FormData();
  form.append('payload_json', JSON.stringify({ embeds: [embed] }));
  for (let i = 0; i < diffs.length; i++) {
    const buf = await readFile(diffs[i]);
    form.append(
      `files[${i}]`,
      new Blob([buf], { type: 'image/png' }),
      path.basename(diffs[i]),
    );
  }

  const res = await fetch(DISCORD_ALERT_WEBHOOK_URL, { method: 'POST', body: form });
  if (!res.ok) {
    throw new Error(`Discord webhook returned ${res.status}: ${await res.text()}`);
  }
  console.log(`notify-visual-diff: alert sent (${diffs.length} attachment(s)).`);
}

main().catch((err) => {
  console.error('notify-visual-diff:', err.message);
  process.exit(1);
});
