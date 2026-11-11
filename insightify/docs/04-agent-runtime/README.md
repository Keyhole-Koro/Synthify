# Agent Runtime

AI が tool を使用してコードや Graph に対する作業を行う実行基盤を扱う。

予定する主文書:

- `agent-run-lifecycle.md` — queue、prepare、run、approval、verify、cancel、recovery
- `local-runner-architecture.md` — local repository、daemon/CLI、control plane 接続
- `tool-policy.md` — filesystem、terminal、network、dependency、secret の capability
- `worktree-and-artifacts.md` — Git worktree、patch、base commit、test Evidence
- `handoff-spec.md` — Run 終了時の構造化成果
- `provider-adapter-spec.md` — capability negotiation、normalized event、provider session mapping

Provider の初期方針は [Technology Stack and Runtime Architecture](../00-overview/technology-stack.md) に定義する。
Codex app-server を first-class provider、Antigravity CLI headless を subscription-oriented secondary provider とし、
Antigravity SDK は BYOK / Vertex 用の後続 adapter とする。

Run の durable state と realtime progress の source of truth を分離する。
