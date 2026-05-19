# Stage / Prod Smoke Test Specification

## Purpose

Stage / prod deploys run smoke tests immediately after the new revision is applied. The goal is to catch obvious deploy breakage before users hit it, without running destructive E2E flows in prod.

This is not a full correctness suite. It is a low-side-effect deploy gate for liveness, readiness, routing, auth gate, and frontend app-shell availability.

## Execution

Backend deploys run `scripts/smoke-test.sh api` and `scripts/smoke-test.sh worker` from `.github/workflows/deploy-backend.yml`.

- GitHub Actions generates an ephemeral `READINESS_API_KEY` with `openssl rand -base64 32`.
- The key is masked with `::add-mask::`.
- Terraform receives the key through `-var="readiness_api_key=..."`.
- API and Worker Cloud Run revisions receive it as `SYNTHIFY_READINESS_KEY`.
- The same key is passed to `scripts/smoke-test.sh` for readiness calls.
- Worker is internal-only, so the workflow installs the `cloud-run-proxy` component and checks it through `gcloud run services proxy`.

Frontend deploys run `scripts/smoke-test.sh frontend` from `.github/workflows/deploy-frontend.yml`. The workflow runs `firebase deploy --json`, extracts the Hosting URL from the deploy result, and falls back to `https://<GCP_PROJECT_ID>.web.app` if the JSON does not contain a URL.

## Checks

API smoke checks:

- `GET /health` returns 2xx without a key.
- `GET /health?ready=1` returns 2xx with `X-Synthify-Readiness-Key`.
- Readiness executes the store `CheckReadiness` dependency check. For Postgres this is `SELECT 1`.
- An unauthenticated Connect RPC to `WorkspaceService/ListWorkspaces` returns `401`, proving the auth gate is active.

Worker smoke checks:

- `GET /health` returns 2xx without a key.
- `GET /health?ready=1` returns 2xx with `X-Synthify-Readiness-Key`.
- Readiness executes the store `CheckReadiness` dependency check. For Postgres this is `SELECT 1`.
- An invalid Connect RPC to `WorkerService/GenerateExecutionPlan` returns `400`, proving the Connect route is alive.

Frontend smoke checks:

- `GET FRONTEND_URL` returns 2xx.
- The response looks like an HTML app shell (`html`, `doctype`, `__next`, or script marker).

## Endpoint Contract

`GET /health` is public liveness:

- No key required.
- Does not check dependencies.
- Intended for basic Cloud Run or external liveness checks.

`GET /health?ready=1` is protected readiness:

- Requires `X-Synthify-Readiness-Key`.
- Missing or wrong key returns `401`.
- Dependency failure returns `503`.
- Success returns `200`.
- Key comparison is constant-time over SHA-256 digests.

If `SYNTHIFY_READINESS_KEY` is empty, readiness fails closed: no request can authorize.

## Non-Goals

- No prod write, create, update, delete, or processing-start flows.
- No Gemini generation call in prod deploy gate.
- No Stripe billing API call in prod deploy gate.
- No Firebase user login flow in prod deploy gate.

Those can be added as stage-only synthetic tests or scheduled monitors with separate credentials and alerting.

## Required Configuration

Backend deploy workflow:

- `GCP_WIF_PROVIDER`
- `GCP_WIF_SA_EMAIL`
- `GCP_PROJECT_ID`
- `GCP_REGION`

Runtime:

- `SYNTHIFY_READINESS_KEY` is injected by Terraform from the deploy-generated `readiness_api_key` variable.
- `DATABASE_DSN` must be valid for readiness to pass in deployed environments.

## Failure Semantics

Any smoke test failure exits non-zero and fails the GitHub Actions job.

Common failures:

- `/health` fails: service is unreachable or not serving HTTP.
- Worker proxy never opens locally: `cloud-run-proxy` is missing or `gcloud run services proxy` cannot reach the service.
- `/health?ready=1` returns `401`: deploy key did not reach the Cloud Run revision or smoke script.
- `/health?ready=1` returns `503`: required dependency check failed, currently DB `SELECT 1`.
- API auth gate check is not `401`: auth middleware may be misconfigured.
- Worker Connect check is not `400`: route or protocol handling may be broken.
- Frontend body check fails: hosting may be serving the wrong artifact.
