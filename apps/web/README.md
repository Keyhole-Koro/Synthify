# Synthify Frontend

The frontend uses Bun as its package manager.

## Development

```sh
bun install
bun run dev
```

The dev server listens on port `5173`.

## Checks

```sh
bun run lint
bun run build
```

## Load testing (many items)

Measuring what happens when a workspace holds a lot of knowledge-tree items:

```sh
bun run bench     # L0: data-layer microbenchmark — no browser, no backend
bun run e2e:perf  # L1: in-browser measurement (DOM, iframes, long tasks, heap)
```

L2 (real API and database) is seeded with `scripts/seed_tree_items.sh` from the
repository root. See [docs/improvements/client-item-load-testing.md](../../docs/improvements/client-item-load-testing.md)
for the knobs, the baseline numbers, and what they mean.

Use `bun add`, `bun remove`, and `bun update` for dependency changes. Do not regenerate `package-lock.json`.

`src/gen/proto/**` and `vender/**` are intentionally excluded from the app ESLint pass. Generated protobuf files and vendored library sources are validated separately from the application code.
