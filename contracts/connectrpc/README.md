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
