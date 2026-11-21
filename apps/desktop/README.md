# Insightify Desktop

Initial Electron vertical slice for the Insightify local-first application.

## Boundaries

- Renderer is sandboxed and has no Node.js integration.
- Preload exposes only the typed API in `@insightify/desktop-bridge`.
- Main owns project paths, SQLite, subprocesses, Codex app-server, and Antigravity CLI.
- The provider switcher exposes Codex and Antigravity through one typed Agent API.
- Codex uses the default stdio JSONL transport with workspace-write and network disabled for the initial run.
- Antigravity uses `agy -p ... --output-format stream-json --sandbox`; headless permission denials are reported by the provider.

## Development

```bash
bun install
bun desktop:dev
```

The scripts call Electron Forge through its programmatic API. Forge 7's CLI system check recognizes npm, Yarn, and pnpm
but not Bun; the programmatic entry keeps Bun as the repository package manager while using the same Forge build pipeline.
The root `which@2` override and explicit `scheduler` dependency keep Forge/Vite resolution deterministic under Bun's
workspace linker.

Typecheck and test:

```bash
bun desktop:typecheck
bun desktop:test
bun desktop:package
```

The FlowFold canvas can also be driven without Electron or an agent CLI:

```bash
bun --cwd apps/desktop preview
```

`preview/` mounts the real renderer against a stubbed preload bridge and a fixed Graph, which is how boundary ports,
Portal folds and the Enter/Leave dive are checked in a browser.

The first launch creates `insightify.sqlite3` through Electron's built-in `node:sqlite` in the application user-data
directory. Repository paths are not returned to the Renderer; the bridge uses opaque project IDs.

## Agent providers

Insightify probes both providers at startup. Install and authenticate the CLI you want to use before launching the app:

```bash
codex --version
agy --version
```

Unavailable providers remain visible but disabled. Antigravity headless mode does not expose interactive approval requests,
so configure scoped permissions in the Antigravity CLI rather than bypassing them from Insightify.
