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

Use `bun add`, `bun remove`, and `bun update` for dependency changes. Do not regenerate `package-lock.json`.

`../../packages/proto-ts/gen/**` and `vender/**` are intentionally excluded from the app ESLint pass. Generated protobuf files and vendored library sources are validated separately from the application code.
