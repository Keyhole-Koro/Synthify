# Post-Deploy Visual QA

## Purpose

After a **stage** frontend deploy, automatically exercise the live site in a real
browser to reduce manual QA. This complements the curl-based
[smoke tests](./stage-prod-smoke-tests.md), which confirm a `200` but cannot see
a blank page, a JS-crashed shell, or unintended UI drift.

Two layers, both in `apps/web/e2e/deploy-smoke.visual.ts`:

- **A — liveness (`@smoke`)**: the landing page mounts, emits no console/page
  errors, and shows the brand chrome + the unauthenticated `Google でログイン`
  CTA.
- **B — visual regression (`@visual`)**: the settled landing is compared against
  a committed baseline screenshot.

Phase 1 covers only the **unauthenticated** surface. Authenticated screens need a
seeded stage QA account and are deferred to Phase 2 (see
`issues/tickets/visual-qa-phase2.md`).

## Execution

The `visual-qa` job in `.github/workflows/deploy-frontend.yml` runs after a
successful stage deploy:

1. Installs deps + the Chromium browser.
2. Runs `bun run e2e:visual`, which sets `E2E_REUSE_SERVER=1` (no local compose
   stack) and points Playwright's `visual` project at the deployed site via
   `E2E_BASE_URL` (the resolved Firebase Hosting URL, passed as a job output).
3. On failure: uploads the diff report + snapshots as the `visual-qa-report`
   artifact and posts a Discord alert (with diff images attached) via
   `scripts/notify-visual-diff.mjs` to `DISCORD_ALERT_WEBHOOK_URL`.

Determinism is enforced by the `visual` project's fixed Desktop Chrome viewport
and the project-wide `toHaveScreenshot` defaults (`animations: 'disabled'`,
`caret: 'hide'`, `maxDiffPixelRatio: 0.01`) in `playwright.config.ts`.

## Baselines

Baselines live in `apps/web/e2e/__screenshots__/` and **must be generated on
Linux** (the CI OS) — snapshots captured on macOS/Windows/WSL will not match CI
rendering. Do not commit locally-generated baselines.

### Capturing / updating a baseline

1. Trigger a stage deploy. The first `visual-qa` run has no baseline, so the
   `@visual` test fails and Playwright writes the fresh baseline into the
   uploaded `visual-qa-report` artifact.
2. Download the artifact, review the captured PNGs, and commit them under
   `apps/web/e2e/__screenshots__/`.
3. The next deploy compares against them.

For an **intended** UI change, regenerate the same way (or run
`bun run e2e:visual:update` in a Linux CI context), review the diff, and commit
the new baseline in the same PR as the UI change. Without this discipline the
`@visual` check goes permanently red and stops being trusted.

### Handling flaky regions

If a live environment surfaces a genuinely non-deterministic region (e.g. a demo
document image that varies), add a `mask:` for it in the `@visual` test and
re-capture the baseline. See the inline note in `deploy-smoke.visual.ts`.

## Required secrets / vars

- `DISCORD_ALERT_WEBHOOK_URL` (secret, stage environment) — reused from the
  readiness monitor; where regression alerts are posted.

## Phase 2 (not in this change)

- Dedicated stage QA account + storageState for authenticated screens.
- Fixed Firestore seed (reset → seed) so list/detail views are deterministic.
- Expand `@visual` coverage to key authenticated screens with timestamp masking.
