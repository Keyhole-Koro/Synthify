# OpenAPI Contracts

OpenAPI contracts for HTTP/JSON route handlers that are not part of the ConnectRPC surface.

## Files

- `monitor.yaml` — internal contract for `apps/monitor/ui` route handlers.

## Generate

From `apps/monitor/ui`:

```sh
bun run gen:openapi
```

The generated TypeScript types are written to `apps/monitor/ui/src/generated/openapi.ts`.
