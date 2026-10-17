# RFC: Synthify ChatGPT-plan-first migration

Status: **Proposed / experimental** (October 2026). This supersedes the *production-default* recommendation in `docs/improvements/local-llm-provider.md` if the team chooses to eliminate Gemini and Stripe; it does not supersede its analysis of the current deployment.

## Goal

Let each eligible user generate Synthify knowledge trees on their own ChatGPT plan without a Synthify AI meter or Stripe subscription. Keep Synthify's document/tree UX and account/workspace authorization semantics, while separating **Synthify identity** (Firebase/DB) from **OpenAI plan consent**. Do not silently bill users or route their requests through a shared provider account.

## Hard constraints (as of 2026-10-17)

1. OpenAI SIWC allows OSS public clients to use OAuth PKCE, dynamic registration (`dynamic_agent_client`, `ext_agent_host_id`, issued `client_id`), and scoped Responses inference. Eligible users grant `chatgpt.tokens.use.direct` separately from identity scopes.
2. Persistent tokens stay on a local user-controlled runtime, never in Cloud Run, Postgres, GCS, Firestore, cookies, JS browser storage, metrics, or build artifacts.
3. Requests originate from that user's local/self-controlled runtime and must arise from their own activity or explicitly authorized automations. Never multiplex another user's jobs or create a generic plan-backed API proxy.
4. Inference uses `POST https://api.openai.com/v1/responses`, `store: false`, `stream: true`; consume `response.completed` and handle `response.failed`, `response.incomplete`, interrupted streams, and usage limits.
5. The user must be able to use the SIWC path at no extra charge from Synthify. Commercial remote/SaaS offering may require partner approval; check this before removing a fallback for hosted users.

Sources: [Sign-in](https://developers.openai.com/siwc/token-sharing-open-source/sign-in), [Responses inference](https://developers.openai.com/siwc/token-sharing-open-source/models-and-inference), [Terms](https://openai.com/policies/sign-in-with-chatgpt-terms/), [Quickstart](https://developers.openai.com/siwc/quickstart).

## Proposed runtime split

```
Synthify web (UI, Firebase account, workspace authorization)
    | user explicitly requests a document/tree job
    v
User-controlled local runtime (OAuth secrets, document processing, GPT inference)
    | responses SSE, store:false, stream:true
    v
OpenAI Responses API (the user's granted plan)
    |
    v
validated tree artifacts -> existing authorized Synthify API -> PostgreSQL/GCS
```

This data flow is a **design target**, not already implemented. Authenticated upload/download, local file staging, secure origin/capability handshakes and provenance checks are required before it may be connected to the remote web UI. Avoid relying on Cloud Run to call `localhost`: in a cloud worker `localhost` is its own container, not the user's laptop.

## Incremental migration (non-destructive)

1. **PoC (this PR):** add `tools/siwc-local-poc` for local OAuth + model listing + streamed text inference. Confirm with an eligible real account. No production configuration changes.
2. **Local worker:** implement local `GenerateText` / `GenerateStructured` equivalents of `apps/worker/pkg/worker/llm/client.go` with structured JSON/schema validation, cancellation, and isolated per-job document staging. Expand tests for grounding and citation quality.
3. **Web bridge:** authenticated pairing between Synthify's logged-in web UI and the local runtime; no remote token transmission. Explicit per-job user action, authenticated workspace read/write permissions, input and output size caps. Define what happens when local runtime is offline. Do not have a cloud tenant trigger another user's local token.
4. **Billing retirement:** remove budget gating from `apps/api/internal/application/document.go` *only after* job ownership and quota semantics have been updated; remove Stripe UI/handlers, usage reporter, pricing, credits, webhook, Terraform secrets; preserve historical invoices/usage audit data via migrations and rollback plan. Keep provider-agnostic operational telemetry without prices.
5. **Gemini retirement:** replace generation, ADK runtime dependencies and `gemini-embedding-2` (requires a separately designed local embedding/search path). Remove Gemini env/Cloud Run secrets and configs when local path reaches parity.
6. **Release gate:** automated text/structured streaming, auth state/nonce, plan scope, refresh/expiry, abort, quota errors, multi-account isolation, CORS/CSRF, workspace authz, file safety, crash/retry, end-to-end tree generation, and human quality eval against baseline; ship only after supported platforms and licensing are decided.

## Explicit non-goals of this PR

- No removal of Gemini, Stripe, Firebase, PostgreSQL, or Cloud Tasks yet.
- No SIWC browser login button, proxy endpoint, or hosted app user-facing enablement.
- No automatic token refresh, Windows credential store, or production local-runtime daemon.

## Acceptance checklist for next implementation PR

- [ ] User runs a local executable and completes SIWC OAuth without any API key.
- [ ] Saved ChatGPT `sub` is bound to the issued client ID and never inferred from email alone.
- [ ] At least one real Responses SSE inference reaches `response.completed`; plan cap fails clearly.
- [ ] One Synthify document turns into a validated knowledge tree using only the local runtime.
- [ ] No token appears in browser/devtools requests, cloud telemetry, server databases, or logs.
- [ ] Stripe billing cannot trigger on the SIWC path, and no credit or usage-price is charged.
- [ ] Remote hosted product and distribution comply with approval and license terms.
