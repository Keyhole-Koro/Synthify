# OpenAPI Contracts

OpenAPI contracts for HTTP/JSON route handlers that are not part of the ConnectRPC surface.

## Files

- `log-viewer.yaml` — internal contract for `apps/log-viewer/ui` route handlers.

## Generate

From `apps/log-viewer/ui`:

```sh
bun run gen:openapi
```

The generated TypeScript types are written to `apps/log-viewer/ui/src/generated/openapi.ts`.
