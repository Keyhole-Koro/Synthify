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
| `dialogue-turn.webm` / `.gif` | `apps/web/e2e/dialogue.spec.ts` | Opening a dialogue from a paper's "+", asking a question, and the answer arriving over the Firestore turn subscription |

The `.gif` is a cropped excerpt of the `.webm` beside it, kept because GitHub
renders a GIF inline in a comment while a raw `.webm` link only downloads. The
crop trades the full canvas for legible text; the `.webm` has the whole frame.

Regenerate one with ffmpeg — a generated palette keeps small text readable,
which a default GIF encode does not:

```bash
ffmpeg -ss 12 -t 13 -i video.webm \
  -vf "crop=380:400:420:20,fps=5,scale=680:-1:flags=lanczos,palettegen=max_colors=64:stats_mode=diff" pal.png
ffmpeg -ss 12 -t 13 -i video.webm -i pal.png \
  -lavfi "crop=380:400:420:20,fps=5,scale=680:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=none" out.gif
```

## Note on local runs

The first run against a freshly started stack can time out: Next.js compiles
routes on first request, and that can outlast the 90s test timeout while the Go
services are also warming. Let the stack settle and run again — the recording
above came from such a second run.

`dialogue.spec.ts` needs `E2E_WORKER_FIXTURE=true` on the worker. There is no
Vertex credential locally, so the worker answers from its fixture generator
instead of a model; the point of the test is that the trigger, the Firestore
subscription and the ContentNode rendering line up.
