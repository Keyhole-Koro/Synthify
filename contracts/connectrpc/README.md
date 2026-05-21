# ConnectRPC Contracts

ConnectRPC contracts are defined with Protocol Buffers.

## Canonical Sources

- Proto files: `./synthify/tree/v1/*.proto`
- Buf module config: `../../buf.yaml`
- Go + TypeScript generation config: `../../buf.gen.yaml`
- TypeScript-only generation config: `../../buf.gen.web.yaml`

## Generate

From the repository root:

```sh
go run github.com/bufbuild/buf/cmd/buf@latest generate
```

Generated outputs:

- Go protobuf and Connect handlers: `../../internal/gen`
- TypeScript protobuf and Connect descriptors: `../../packages/proto-ts/gen`
