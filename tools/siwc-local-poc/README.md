# Synthify local SIWC proof of concept

**Experimental; not wired into the Synthify web app, API, or worker.** This is a user-operated CLI for checking whether *your own* eligible ChatGPT plan can complete a Responses API request. There is no API key, Gemini fallback, cloud token store, or shared inference proxy.

## Try it (macOS / Linux, Node 22+)

```bash
cd tools/siwc-local-poc
npm install
node siwc.mjs login
node siwc.mjs models
node siwc.mjs ask <model-slug> 'Summarize the benefits of a knowledge tree in one sentence.'
node siwc.mjs logout
npm test
```

Use a model `slug` shown by `models`; model catalog membership is not an entitlement guarantee. `login` opens the system browser and listens on an ephemeral `127.0.0.1` loopback port at `/auth/callback`. First login uses dynamic OAuth client registration. On later login, the issued client ID and host ID are reused. The client does not ask for your ChatGPT password or session cookie.

## Limitations and security

- **Only CLI requests are supported.** This is not a bridge for browser/remote users and must not be deployed to Cloud Run or exposed as a public HTTP API. It must never use one user's plan for another user's jobs.
- Supported OS: macOS/Linux only. Windows needs secure credential handling before support; no plain Windows file store is implemented.
- OAuth tokens and basic account metadata are stored locally in `~/.config/synthify/siwc-poc.json` (mode 0600). Delete that file to remove local tokens; use `logout` to attempt remote revocation. Never add tokens to your repository, browser storage, telemetry, or logs. The authorization URL printed by this PoC deliberately omits `id_token_hint`.
- This version uses one saved account per local host. It does **not** do automatic OAuth refresh or coordinate concurrent processes. Re-run `login` when the access token expires. Multi-account switching and cross-process refresh locking are required for product integration.
- `ask` enforces `store: false` and `stream: true` and treats completion as successful only after `response.completed`. Streaming text is printed locally; a partial response can appear before an error or usage limit.
- Not integrated with Synthify document upload, knowledge tree, roles, Firestore progress, or billing. No production feature flag is enabled. No actual ChatGPT login/inference was performed while preparing this change (requires an eligible user session).
- Verify licensing and OpenAI partner requirements before general SaaS deployment. The OSS plan-usage route is for user-controlled runtimes; adding a button to remote Web/Cloud Run alone does not grant a right to proxy subscription usage.

Official docs: [OSS sign-in](https://developers.openai.com/siwc/token-sharing-open-source/sign-in), [models/inference](https://developers.openai.com/siwc/token-sharing-open-source/models-and-inference), [SIWC terms](https://openai.com/policies/sign-in-with-chatgpt-terms/).
