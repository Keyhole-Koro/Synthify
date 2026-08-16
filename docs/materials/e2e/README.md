# E2E recordings

Videos captured from passing e2e runs, kept so a UI change can be watched rather
than inferred from assertions.

Playwright keeps video only `retain-on-failure` by default, which is right for
CI: recording every green run is pure storage. `E2E_RECORD=1` switches video and
trace to `on` so a passing run leaves a recording behind.

```bash
cd apps/web
bun run e2e:record                      # boots the local stack
E2E_REUSE_SERVER=1 bun run e2e:record   # reuses a stack that is already up
```

Output lands in `apps/web/test-results/<test>/video.webm` (gitignored). Copy the
ones worth keeping here.

| File | Test | Shows |
|---|---|---|
| `paper-nested-open-close.webm` | `apps/web/e2e/paper.spec.ts` | Uploading a document, then opening nested papers, following in-content links, and restoring the open set across a reload |

## Note on local runs

The first run against a freshly started stack can time out: Next.js compiles
routes on first request, and that can outlast the 90s test timeout while the Go
services are also warming. Let the stack settle and run again — the recording
above came from such a second run.
