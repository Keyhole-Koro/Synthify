# ConnectRPC Contracts

ConnectRPC contracts are defined with Protocol Buffers.

## Canonical Sources

- Proto files: `./synthify/{app,admin,executor,worker,localprovider}/v1/*.proto`
- Buf module config: `../../buf.yaml`
- Go + TypeScript generation config: `../../buf.gen.yaml`
- TypeScript-only generation config: `../../buf.gen.web.yaml`
- Local-provider Go + Python generation config: `../../buf.gen.local-provider.yaml`

## Generate

From the repository root:

```sh
go run github.com/bufbuild/buf/cmd/buf@v1.72.0 generate
go run github.com/bufbuild/buf/cmd/buf@v1.72.0 generate --template buf.gen.local-provider.yaml
```

Generated outputs:

- Go protobuf and Connect handlers: `../../internal/gen`
- TypeScript protobuf and Connect descriptors: `../../apps/web/src/gen/proto`
- Python protobuf and Connect stubs: `../../apps/local-provider/src`

## Request validation (protovalidate)

What a request must contain is declared on the request messages with
[protovalidate](https://protovalidate.com) rules, and nowhere else:

- `(buf.validate.field).required = true` for IDs that must be present.
- `(buf.validate.field).string.(synthify.validate.v1.not_blank) = true` for
  values the server trims (names, emails, tokens); defined once in
  `./synthify/validate/v1/rules.proto`.
- Ranges and cross-field conditions with the usual protovalidate rules or CEL.

The api and worker Connect servers run every request through
`internal/platform/transport/rpcvalidate`, which answers a broken request with
`invalid_argument` and the violations before the handler runs. Handlers do not
repeat these checks; they keep only checks that depend on authorization or on
other requests (for example the scope-dependent log searches in `JobService`).
