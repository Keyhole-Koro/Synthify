# Insightify Technology Stack and Runtime Architecture

Status: Accepted for MVP  
Last updated: 2026-08-28  
Owners: Insightify core team  
Related: [System Design](system-design.md), [FlowFold Interaction Specification](../01-flowfold/interaction-spec.md)

## 0. Decision Summary

Insightify は、単一の Web application でも、単一の desktop binary でもない。

次の 3 層に分ける。

1. **Insightify Desktop** — FlowFold UI、local repository、Agent、terminal、local database を所有する。
2. **Local Agent Runtime** — Codex app-server や Antigravity CLI などを adapter 経由で実行する。
3. **Synthify Hub** — 認証、共有 Graph、Thread、presence、review、同期を Internet 越しに提供する。

初期技術選定は次の通りとする。

| Area | Decision | Initial choice |
|---|---|---|
| Desktop shell | Web UI と privileged runtime を process 分離する | Electron |
| Desktop renderer | Web と共有可能な UI package | React 19 + TypeScript + Vite |
| Flow renderer | Room 単位の DOM node + SVG edge | custom FlowFold renderer |
| Desktop privileged layer | subprocess、filesystem、Git、keychain、DB のみを所有 | Electron Main + Utility Process |
| Local persistence | offline-first working store | SQLite, WAL mode |
| Local SQLite driver | driver 境界で隔離 | Electron内蔵の `node:sqlite` initially |
| Cloud API | 既存 Synthify backend を拡張 | Go + ConnectRPC + Protobuf |
| Cloud durable DB | 既存資産を優先 | PostgreSQL initially |
| Global DB option | multi-region 要件成立後に再評価 | CockroachDB candidate |
| Durable sync | operation log + revision + outbox | ConnectRPC API |
| Realtime fanout | presence、cursor、run progress | WebSocket gateway |
| Primary AI provider | subscription login、approval、usage を統合 | Codex app-server |
| Secondary AI provider | signed-in product quota を利用 | Antigravity CLI headless |
| API-billed AI option | BYOK / Vertex 向け | Antigravity SDK sidecar, later |
| Shared contracts | frontend/backend/provider 間の schema | Protobuf + Zod at UI boundary |
| Package manager | repository に合わせる | Bun workspaces |
| Testing | domain、UI、desktop integration | Vitest + Playwright + Go tests |

MVP で CockroachDB、CRDT、remote cloud runner、独自 terminal emulator、独自 model gateway を同時に導入しない。
Insightify の差別化は FlowFold と anchored AI collaboration であり、基盤技術の数ではない。

## 1. Product Boundary

### 1.1 Local-first, not local-only

Insightify Desktop は network がなくても次を実行できる。

- project を開く
- Room を移動する
- Graph を編集する
- Thread を作成する
- local Agent Run を開始する
- ChangeSet を確認し、local branch に適用する
- local history を検索する

Synthify Hub に接続すると、次が追加される。

- team member と Graph、Thread、Decision を共有する
- presence と cursor を表示する
- ChangeSet を review する
- Branch、lease、merge 状態を同期する
- remote web client から read/comment/review する
- desktop 上の Agent Run の redacted progress を共有する

Internet connection は「アプリを使う条件」ではなく、「協業範囲を広げる能力」とする。

### 1.2 Subscription credentials stay local

ChatGPT、Google、Git provider、cloud CLI の credential は Desktop の所有物である。
Synthify Hub は credential を保存せず、代理実行もしない。

Hub へ送信可能なものは明示的に分類する。

| Data | Default |
|---|---|
| Provider access/refresh token | never upload |
| Repository file contents | local only |
| Raw terminal output | local only |
| Raw model event payload | local only |
| Graph operations | sync |
| Thread messages | sync when thread is shared |
| Structured run status | sync |
| Patch / diff | opt-in per ChangeSet |
| Test evidence | opt-in; summary by default |
| Secret-like text detected in output | block and require review |

これにより、subscription usage を使う local execution と、Internet collaboration を同一製品内で両立する。

## 2. Deployment Topology

```text
┌──────────────────────── Insightify Desktop ────────────────────────┐
│                                                                    │
│  ┌──────────── Renderer (sandboxed) ────────────┐                  │
│  │ FlowFold / Threads / Review / Split View     │                  │
│  │ React + TypeScript + local packaged assets   │                  │
│  └──────────────────┬───────────────────────────┘                  │
│                     │ narrow typed IPC                              │
│  ┌──────────────────▼───────────────────────────┐                  │
│  │ Electron Main                               │                  │
│  │ window / permissions / process supervision  │                  │
│  └───────┬──────────────┬──────────────┬────────┘                  │
│          │              │              │                            │
│  ┌───────▼──────┐ ┌─────▼──────┐ ┌────▼─────────┐                 │
│  │ Runtime Host │ │ SQLite     │ │ Git / PTY    │                 │
│  │ utility proc │ │ local store│ │ worktrees    │                 │
│  └───────┬──────┘ └────────────┘ └──────────────┘                 │
│          │ stdio / JSONL / stream-json                             │
│     ┌────▼─────────────┬───────────────────────┐                   │
│     │ codex app-server │ agy headless         │ optional SDK proc  │
│     └──────────────────┴───────────────────────┘                   │
└───────────────────────────┬────────────────────────────────────────┘
                            │ HTTPS / WSS, outbound only
┌───────────────────────────▼ Synthify Hub ──────────────────────────┐
│ Firebase Auth │ Go API │ Sync/Presence Gateway │ Review Web UI     │
│                PostgreSQL initially / CockroachDB candidate        │
└────────────────────────────────────────────────────────────────────┘
```

### 2.1 Trust boundaries

境界は UI convenience ではなく security boundary として扱う。

- Renderer は filesystem、shell、environment variable、provider token に直接触れない。
- Preload は汎用 `execute(command)` を公開しない。
- Renderer から Main へは discriminated union の command のみ送る。
- Main は request origin、project ID、path scope、run state を再検証する。
- Provider process は project worktree を working directory として起動する。
- Agent の writable root は run ごとに固定する。
- remote content を privileged Renderer に読み込まない。
- Hub から届いた文字列を shell command に変換しない。

## 3. Why Electron for the MVP

### 3.1 Decision

MVP desktop shell は Electron とする。

理由は bundle size ではなく、Insightify の中心が次の local process orchestration にあるためである。

- Codex app-server の長寿命 stdio session
- Antigravity CLI の長寿命 stream session
- Git、worktree、diff、test command
- PTY と terminal lifecycle
- OAuth callback と OS keychain
- SQLite writer
- crash recovery と subprocess supervision

Electron Main は Node.js runtime であり、これらを TypeScript から直接扱える。
また Renderer は通常の Web UI なので、FlowFold を Synthify の review web client と package 共有できる。

### 3.2 Electron process policy

Electron を採用する代わりに、次を mandatory とする。

```ts
type RequiredWebPreferences = {
  nodeIntegration: false;
  contextIsolation: true;
  sandbox: true;
  webSecurity: true;
};
```

さらに以下を守る。

- packaged local assets のみを primary window に load する
- strict Content Security Policy を設定する
- navigation と new-window creation を deny-by-default にする
- `shell.openExternal` の URL scheme と host を allowlist する
- IPC sender と payload を validation する
- Main process に重い parsing、layout、provider normalization を置かない
- provider normalization は Utility Process へ隔離する
- Electron は supported stable release を追随する

Electron 公式の process model は Main、sandboxed Renderer、Utility Process の分離を前提にしている。
公式 security checklist も Node integration 無効化、context isolation、sandbox、IPC sender validation を要求している。

References:

- [Electron Process Model](https://www.electronjs.org/docs/latest/tutorial/process-model)
- [Electron Security](https://www.electronjs.org/docs/latest/tutorial/security)
- [Electron Distribution Overview](https://www.electronjs.org/docs/latest/tutorial/distribution-overview)

### 3.3 Why not Tauri first

Tauri 2 も有力であり、shell plugin は desktop で child process を spawn できる。
ただし MVP では次の追加コストが発生する。

- TypeScript domain と Rust command layer の二重境界
- provider event の高頻度 stream bridge
- PTY、Git、process supervision の Rust 実装または plugin 選定
- existing TypeScript prototype からの距離

したがって「小さい配布物」という利点より、Agent runtime を早く安全に検証できることを優先する。

Tauri を永久に除外する判断ではない。Phase 2 の実測で Electron の idle memory、startup time、update size が
product requirement を満たさない場合に再評価する。

Reference: [Tauri Shell plugin](https://v2.tauri.app/plugin/shell/)

### 3.4 Packaging

初期 packaging は Electron Forge を使い、macOS、Windows、Linux を別々の CI job で build する。

- macOS: signed + notarized DMG/ZIP
- Windows: signed installer
- Linux: AppImage または deb を先行
- auto-update: staged rollout、rollback metadata、minimum supported schema version を持つ

Provider CLI は原則として bundle しない。Desktop 起動時に検出し、version と capability を表示する。
これにより provider の license、update cadence、login state を Insightify distribution から分離できる。

## 4. Desktop Frontend

### 4.1 Stack

| Concern | Choice |
|---|---|
| UI | React 19 |
| Language | TypeScript strict mode |
| Bundler | Vite |
| Styling | Tailwind CSS 4 + CSS custom properties |
| Animation | Framer Motion, navigation transitions only |
| Boundary validation | Zod |
| Unit/component test | Vitest + Testing Library |
| Desktop E2E | Playwright Electron automation |

Next.js は Synthify Hub の Web UI では維持するが、Desktop Renderer には埋め込まない。
Desktop は SSR、server actions、route server を必要としないため、Vite の local static bundle の方が境界が明確である。

### 4.2 FlowFold rendering architecture

FlowFold renderer は既存 `paper-in-paper` をそのまま製品依存にしない。
既存実装は interaction と layout 仮説の reference/prototype として扱い、以下の境界で再構成する。

```text
flowfold-domain       immutable graph, commands, invariants
flowfold-layout       Room-local layout, boundary ports, routing
flowfold-viewport     camera, semantic zoom, visible set
flowfold-react        DOM nodes, SVG edges, selection, gestures
flowfold-collab       remote cursors, leases, change overlays
```

初期 renderer は DOM + SVG とする。

- Node、Portal、Thread card は DOM
- Edge、selection lasso、boundary projection は SVG overlay
- layout calculation は Web Worker
- Room ごとに visible set を計算し、非表示 Room は mount しない
- Peek は child Room 全体を再帰 mount せず、snapshot representation を使う
- Portal の outer size は child layout size から独立させる

Canvas/WebGL は初期採用しない。Room と Semantic Zoom によって同時表示数を制限する設計なので、まず DOM の
accessibility、text editing、inspection、testability を取る。1 Room の表示対象が 500 nodes を超えて interaction
budget を破る場合のみ PixiJS/WebGL layer を検討する。

### 4.3 State ownership

React component tree を source of truth にしない。

| State | Owner |
|---|---|
| Graph entities and revisions | local repository / SQLite |
| Pending graph edits | domain command session |
| Camera, selection, open Peek | viewport store |
| Server cache | sync client cache |
| Provider run state | runtime event reducer |
| Presence | ephemeral collaboration store |
| Unsaved text composition | component-local until commit |

Domain command と reducer は pure TypeScript にし、React から独立して test する。
UI store は `useSyncExternalStore` compatible な小さい external store とし、domain entity を複製しない。

## 5. Local Persistence

### 5.1 SQLite is the desktop source of truth

Desktop の working state は SQLite に保存する。

SQLite は embedded であり、network service を必要とせず、project ごとの offline operation と atomic transaction に合う。
WAL mode では reader と writer が並行できる。ただし WAL は network filesystem 用ではないため、DB file は必ず local
application data directory に置く。repository 配下や同期フォルダには置かない。

Reference: [SQLite Write-Ahead Logging](https://sqlite.org/wal.html)

### 5.2 SQLite access policy

- DB connection を Runtime Host 内の単一 owner が管理する
- WAL mode を使う
- explicit transaction と foreign key を有効にする
- schema migration は forward-only migration file で管理する
- DB file、WAL、SHM を一組として backup する
- long read transaction による checkpoint starvation を監視する
- repository path は canonicalize して project record に保存する
- large blob、repository snapshot、terminal transcript は DB に無制限保存しない

Electron が内蔵する `node:sqlite` を initial driver とする。native addon の再 build や unpack を避けられるが、application code は repository 境界の外で driver を直接 import しない。
Node built-in `node:sqlite` は 2026-08 時点で release candidate のため、stable になった時点で adapter 差し替えを検討する。

### 5.3 Local schema groups

```text
projects
project_mounts
scopes
nodes
ports
edges
graph_revisions
graph_operations
threads
thread_messages
thread_memories
agent_sessions
agent_runs
run_events
changesets
artifacts
sync_outbox
sync_inbox
sync_cursors
provider_installations
```

`run_events` は replay/debug 用に bounded retention を持つ。provider-native raw event は encrypted local storage にのみ置き、
normalized event と別 column/table にする。

### 5.4 Secrets

次は SQLite に保存しない。

- OAuth refresh token
- provider API key
- repository credential
- signing key

OS keychain を使用し、DB には opaque key reference のみ置く。keychain が利用できない環境では persistent login を無効にし、
plaintext fallback は行わない。

## 6. Cloud Persistence and CockroachDB

### 6.1 Decision for MVP

Synthify Hub の MVP durable database は、現行 repository の PostgreSQL、pgx、sqlc を継続利用する。

CockroachDB は有力だが、Desktop local DB の代替ではない。また現在の Synthify backend は既に PostgreSQL repository、
transaction boundary、migration、pgvector integration を持つ。MVP で database migration まで同時に行うと FlowFold の検証速度を落とす。

### 6.2 When CockroachDB becomes the right choice

次のうち複数が実要件になった時点で CockroachDB ADR を採決する。

- active users と project data を複数 region に配置する
- region failure survival が product SLO に含まれる
- global organization に対し data locality を選ばせる
- single-region PostgreSQL の failover/scale が実測上の制約になる
- CockroachDB の運用費を含む TCO が PostgreSQL より合理的になる

CockroachDB は PostgreSQL wire protocol と多くの SQL syntax をサポートするが、完全な PostgreSQL clone ではない。
また default の `SERIALIZABLE` transaction では application が transaction retry を扱う場合がある。

References:

- [CockroachDB Developer Basics](https://www.cockroachlabs.com/docs/stable/developer-basics.html)
- [CockroachDB Multi-Region Overview](https://www.cockroachlabs.com/docs/stable/multiregion-overview.html)

### 6.3 Cockroach-ready rules now

PostgreSQL を使いながら、Insightify の新規 collaboration schema は次を守る。

- identifier は ULID/UUID。database sequence を外部 ID にしない
- transaction body は retry-safe、idempotent にする
- request と Graph operation に idempotency key を持たせる
- durable event と side effect の間は transactional outbox を使う
- advisory lock を domain correctness に使わない
- `LISTEN/NOTIFY` を durable delivery に使わない
- PostgreSQL extension 依存を core collaboration table に入れない
- current revision を条件にした compare-and-swap update を使う
- region/data residency を後付け可能な tenant/project field として持つ

この制約により、CockroachDB compatibility spike を独立して実施できる。

### 6.4 Database is not the sync protocol

Desktop client を PostgreSQL/CockroachDB に直接接続しない。Cloud database replication を local-first sync として利用しない。

Desktop と Hub の間では、認可済み domain operation を交換する。

```ts
type GraphOperationEnvelope = {
  operationId: string;      // ULID, idempotency key
  projectId: string;
  actorId: string;
  deviceId: string;
  baseRevision: string;
  clientSequence: number;
  occurredAt: string;
  operation: GraphOperation;
};
```

Server は project ごとの authoritative revision と server sequence を発行する。Desktop は outbox に書いてから送信し、ack 後に
cursor を進める。reconnect 時は ack 済み sequence 以降を再取得する。

## 7. Synthify Hub

### 7.1 Reuse instead of rewrite

現行 Synthify の次の資産を control plane として再利用する。

- Go API
- ConnectRPC + Protobuf contract
- Firebase Authentication
- PostgreSQL repository layer
- Cloud Tasks / worker infrastructure
- Next.js Web UI
- Firestore job-status integration

ただし既存のすべてを FlowFold の source of truth にしない。

### 7.2 New Hub responsibilities

```text
Identity and membership
  organizations / projects / roles / devices

Durable collaboration
  graph revisions / operations / threads / decisions / changesets

Realtime collaboration
  presence / cursors / viewport hints / run progress

Review
  proposed graph operations / patches / evidence / approvals

Synchronization
  pull cursor / push outbox / conflict response / snapshots
```

Repository code、provider token、local filesystem は responsibility に含めない。

### 7.3 Protocols

| Path | Protocol | Durability |
|---|---|---|
| Authenticated commands and queries | ConnectRPC over HTTPS | durable via DB |
| Snapshot and operation pull | ConnectRPC | durable via DB |
| Presence and cursor | WebSocket | ephemeral |
| Run progress | WebSocket + durable milestones | mixed |
| Large opted-in artifact | signed object-storage upload | durable |

WebSocket connection が切れても correctness を失わない。Graph mutation は必ず ConnectRPC command と DB transaction を通る。
WebSocket は low-latency projection であり、source of truth ではない。

MVP の fanout は Go process 内 hub から始められる。複数 instance が必要になった時点で managed pub/sub または NATS を導入する。
Firestore は既存 job progress の互換経路として維持できるが、FlowFold graph の authoritative store にはしない。

### 7.4 Web companion

Synthify Web は Desktop の完全版ではなく、collaboration companion とする。

Web で可能:

- FlowFold の閲覧と navigation
- comment / Thread reply
- review / approve / reject
- presence
- shared Decision の編集

Web で初期提供しない:

- local repository tool execution
- subscription-authenticated Agent Run
- arbitrary terminal
- local worktree management

FlowFold domain/layout/react package を共有し、Desktop 固有 API は provider interface の外に閉じ込める。

## 8. Agent Provider Architecture

### 8.1 Capability-based adapter

すべての provider を最小公分母に押し込めない。adapter は capability を宣言する。

```ts
type AgentCapabilities = {
  managedSubscriptionLogin: boolean;
  accountStatus: boolean;
  rateLimits: "structured" | "text" | "none";
  persistentThreads: boolean;
  forkThread: boolean;
  resumeThread: boolean;
  interactiveApprovals: boolean;
  sandboxPolicy: "structured" | "provider-config" | "none";
  structuredOutput: boolean;
  streamedToolEvents: boolean;
  cancelTurn: boolean;
};

interface AgentProvider {
  probe(): Promise<ProviderInstallation>;
  capabilities(): AgentCapabilities;
  account(): Promise<AccountState>;
  listSessions(query: SessionQuery): Promise<AgentSession[]>;
  startSession(input: StartSessionInput): Promise<AgentSession>;
  startRun(input: StartRunInput): AsyncIterable<AgentEvent>;
  cancelRun(runId: string): Promise<void>;
  respondToApproval?(request: ApprovalResponse): Promise<void>;
}
```

UI は `interactiveApprovals: false` の provider に approval button を偽装表示しない。
rate limit が machine-readable でない場合、推測した percentage を表示しない。

### 8.2 Normalized events

Provider event は次へ normalize する。

```text
RunStarted
AssistantTextDelta
ReasoningSummaryDelta
ToolCallProposed
ApprovalRequested
ToolCallStarted
ToolOutputDelta
ToolCallFinished
FilePatchProposed
UsageUpdated
RunWarning
RunFailed
RunCompleted
```

各 event は `projectId`, `threadId`, `runId`, `provider`, `providerSessionId`, `sequence`, `timestamp` を持つ。
unknown provider event は捨てず、local raw log へ保存するが Cloud には送らない。

## 9. Codex Integration

### 9.1 First-class provider

Codex は `codex app-server` を Desktop Main から child process として起動し、default の stdio JSONL transport を使う。
experimental WebSocket transport には依存しない。

app-server は rich client 向けに次を公開する。

- ChatGPT managed login
- account と plan information
- rate-limit snapshot と update
- thread start/list/read/archive
- turn start/cancel
- streamed agent/tool events
- command/file-change approval request
- cwd、approval policy、sandbox policy、writable root

このため、ChatGPT subscription usage を Insightify UI から利用する目的に最も一致する。

Reference: [Codex app-server](https://learn.chatgpt.com/docs/app-server)

### 9.2 Process lifecycle

```text
probe `codex --version`
  -> spawn `codex app-server`
  -> initialize
  -> account/read
  -> account/rateLimits/read
  -> thread/start or thread/resume
  -> turn/start
  -> normalize notifications and approval requests
  -> turn completion
  -> retain process while project is active
  -> graceful shutdown, then forced kill on timeout
```

app-server process は project window ごとではなく Desktop runtime ごとに pool する。ただし thread と cwd の関連を explicit に保持し、
別 project の event を混在させない。

### 9.3 Safety mapping

| Insightify concept | Codex app-server field/behavior |
|---|---|
| Project mount | `cwd` |
| Run writable set | sandbox `writableRoots` |
| Network capability | sandbox network policy |
| Human approval | server-initiated approval request |
| Thread | app-server thread |
| Agent Run | turn |
| Usage indicator | account rate limits |
| Usage exhaustion | usage-limit error |

Insightify ChangeSet approval と provider tool approval は別物である。

- tool approval: command/file operation を実行してよいか
- ChangeSet approval: 実行結果を shared Graph/branch に採用してよいか

前者が approve 済みでも、後者は自動 approve しない。

### 9.4 Compatibility

app-server protocol は version と capability を起動時に記録する。

- minimum supported Codex version を定義する
- unknown notification を tolerate する
- required method がなければ provider を degraded にする
- auth token file を直接読み取らない
- app-server の managed login flow を利用する

## 10. Antigravity Integration

### 10.1 Product quota and SDK billing are different paths

Antigravity は二つの adapter を区別する。

1. **Antigravity CLI adapter** — signed-in Antigravity product/CLI account を使う。subscription usage 目的はこちら。
2. **Antigravity SDK adapter** — Gemini API key または Vertex/ADC を使う。BYOK/API billing 目的。

公式 SDK quickstart は `GEMINI_API_KEY`、または Vertex AI credentials を要求する。したがって SDK を consumer subscription の
usage path とみなさない。

References:

- [Antigravity SDK overview](https://antigravity.google/docs/sdk/overview/)
- [Antigravity plans and quotas](https://antigravity.google/docs/plans)

### 10.2 CLI adapter

CLI は `agy` headless の persistent stream mode を使い、stdin/stdout の `stream-json` を Runtime Host が管理する。
Google account login は CLI 自身に任せ、Insightify は credential file を読まない。

利用可能な能力:

- signed-in account
- streamed text/tool events
- conversation ID と resume
- cwd-scoped conversation
- fork command
- token usage events

References:

- [Antigravity CLI installation and authentication](https://antigravity.google/docs/cli/install/)
- [Antigravity CLI headless mode](https://antigravity.google/docs/cli/headless/)
- [Antigravity CLI conversations](https://antigravity.google/docs/cli/conversations/)

### 10.3 CLI limitations

headless permission flow は Codex app-server のような interactive approval request protocol ではない。
approval-required tool は事前 permission rule がなければ soft-deny される。このため MVP では:

- Antigravity Run は isolated worktree で実行する
- permission profile を run 開始前に明示する
- network と writable paths を最小化する
- dangerous skip-permission flag を通常 UI から提供しない
- interactive approval を capability false として表示する

`/usage` は CLI command で確認可能だが、stream session 内 API ではない。structured quota percentage として brittle parsing せず、
separate usage command の raw result を provider status panel に表示する。

Reference: [Antigravity CLI usage command](https://antigravity.google/docs/cli/commands/usage/)

### 10.4 SDK adapter later

SDK adapter は Python sidecar として実装し、Desktop Node process に Python library を埋め込まない。

```text
Electron Runtime Host
  <-> local authenticated IPC / stdio
Python Provider Sidecar
  <-> Antigravity SDK
```

SDK の policy、lifecycle hook、session persistence は provider abstraction と相性がよいが、API billing であることを UI に明記する。

## 11. Session Split, Merge, and Lock

Provider session と Insightify Thread/Branch を同一 ID にしない。

```text
Insightify Thread
  ├── Context Checkpoint A
  ├── Provider Session codex:thread-123
  ├── Agent Run run-01
  └── Fork
      ├── Branch B + Provider Session codex:thread-456
      └── Branch C + Provider Session agy:conversation-789
```

### 11.1 Split

Split は次を atomic に作る。

- new Insightify Branch
- Context Checkpoint
- provider session fork、または compiled context から新 session
- base Graph revision
- read/write intent

Provider が native fork を持たない場合、Context Compiler が structured checkpoint を新 session の初期 context に渡す。

### 11.2 Merge

AI conversation history 自体は merge しない。merge 対象は構造化された成果である。

- Graph operations
- Patch
- Decision
- Thread Memory
- Evidence
- Open Questions

merge 後の primary Thread には merge summary と provenance を追加する。

### 11.3 Lock and lease

長時間の排他 lock ではなく、Hub が短い renewable lease を発行する。

- lease は Scope、Node set、Artifact path set のいずれか
- Desktop disconnect 時に expiry する
- provider process が生きていても lease expiry 後は shared apply できない
- local work は失わず、新 revision へ rebase する
- read と proposal は lock なしで可能

## 12. Collaboration Conflict Model

MVP では Graph 全体を CRDT にしない。

Graph operation は semantic intent を持つため、base revision に対する optimistic validation を行う。

```text
CreateNode
DeleteNode
MoveNode
SetNodeContract
CreateEdge
ReconnectEdge
SetPortalMapping
AttachThread
```

Server は conflict を operation level で返す。

- same field changed
- target deleted
- port contract changed
- edge invariant violated
- branch lease lost
- stale base revision but auto-rebasable

Presence、cursor、selection は ephemeral LWW でよい。将来同一 rich-text body の simultaneous editing が必要になった場合だけ、
Yjs 等の CRDT を限定導入する。

## 13. Repository and Package Layout

既存 monorepo を段階的に次へ拡張する。

```text
apps/
  desktop/                    Electron application
  web/                        Synthify/Insightify Hub web companion
  api/                        existing Go control plane
  worker/                     existing async worker
  local-provider/             existing Python service; future SDK host candidate

packages/
  flowfold-domain/            pure TypeScript entities and commands
  flowfold-layout/            Room-local layout engine
  flowfold-react/             shared Desktop/Web renderer
  insightify-protocol/        generated TS contracts and runtime schemas
  agent-runtime/              provider-neutral types and event reducer
  agent-codex/                Electron-main/utility-only adapter
  agent-antigravity-cli/      Electron-main/utility-only adapter
  desktop-bridge/             narrow IPC contracts

proto/
  insightify/graph/v1/
  insightify/collaboration/v1/
  insightify/sync/v1/
  insightify/agent/v1/
```

`agent-codex` と `agent-antigravity-cli` は Web bundle から import 不可能にする。package export と lint rule で強制する。

`insightify/` documentation hierarchy は当面維持し、implementation package を作る時点で repository-level build system に接続する。

## 14. API and Contract Strategy

### 14.1 Protobuf at service boundaries

Hub API、sync operation、durable run milestone は Protobuf を source of truth とする。
既存 ConnectRPC generator と validation pipeline を再利用する。

### 14.2 TypeScript domain types internally

Renderer 内の high-frequency layout data を Protobuf message のまま操作しない。
boundary で domain type に変換し、branded ID と exhaustive union を使う。

### 14.3 IPC contract

Desktop IPC は method name + arbitrary JSON ではなく、versioned command/event union とする。

```ts
type DesktopCommand =
  | { type: "project.open"; pathToken: string }
  | { type: "agent.run.start"; input: StartRunInput }
  | { type: "agent.approval.respond"; input: ApprovalResponse }
  | { type: "changeset.apply"; changeSetId: string }
  | { type: "sync.connect"; projectId: string };
```

Renderer が raw filesystem path や raw shell argv を任意に指定できる command を作らない。

## 15. Observability and Privacy

### 15.1 Local diagnostics

- structured JSON logs
- process start/exit and version
- provider protocol sequence and latency
- SQLite transaction and migration result
- layout duration and visible node count
- sync outbox depth and cursor lag
- approval latency

Raw prompt、model output、file content、diff は telemetry に含めない。

### 15.2 Cloud telemetry

Cloud へ送る default metric は metadata のみとする。

- app version / OS / architecture
- crash fingerprint
- provider kind, not account identity
- run status and duration
- operation count and conflict type
- Room render performance bucket

support bundle export は user preview を必須にする。

## 16. MVP Delivery Slices

### Slice A — Desktop shell spike

Goal: architecture risk を 1 週間以内に潰す。

- Electron Renderer/Main/Utility Process 分離
- narrow typed IPC
- local SQLite migration
- existing FlowFold prototype を一画面表示
- installed `codex` detection
- app-server initialize/account/thread/turn
- streamed text event 表示
- graceful cancel/shutdown

Exit criteria:

- Renderer から Node API に触れない
- app-server crash 後に UI を reload せず再起動できる
- restart 後に local Thread と Graph が復元する
- minimal signed development package を 2 OS で起動できる

### Slice B — Local FlowFold + Codex vertical path

- Room/Portal/Peek/Split View
- Node anchored Thread
- Context Compiler v0
- Codex approval UI
- isolated Git worktree
- patch preview
- ChangeSet accept/reject
- usage indicator

Exit criteria:

- 一つの Node から Run を開始し、patch と test evidence を戻せる
- 別 Room の context が明示なしに混入しない
- approval なしの out-of-scope write を拒否する

### Slice C — Synthify Hub collaboration

- Firebase device login
- project membership
- Graph operation push/pull
- Thread sync
- presence/WebSocket
- web review companion
- reconnect/outbox recovery

Exit criteria:

- 二台で同じ project を開き、offline edit 後に再同期できる
- conflict が silent overwrite にならない
- provider credential と repository contents が Hub に送信されない

### Slice D — Antigravity CLI

- installation/login probe
- persistent headless stream adapter
- conversation resume/fork mapping
- permission profile UI
- raw usage panel
- capability differences UI

### Slice E — Branching and multi-agent work

- split into worktrees
- read/write intent and lease
- parallel Agent Runs
- structured handoff
- semantic merge

CockroachDB spike は Slice C の負荷、region、SLO が明らかになってから行う。

## 17. Required Spikes and Measurements

### 17.1 Codex app-server spike

Measure:

- startup/auth latency
- event throughput
- approval round trip
- cancel reliability
- reconnect/recovery behavior
- version skew
- rate-limit update behavior

### 17.2 Antigravity CLI spike

Measure:

- stream-json stability
- multiple concurrent conversations
- permission-denied event shape
- interrupt/cancel semantics
- raw usage output stability
- credential reuse across process restart

### 17.3 FlowFold renderer spike

Test datasets:

- 50 visible / 1,000 total nodes
- 200 visible / 10,000 total nodes
- Portal depth 10
- cross-boundary edges 200
- Peek 3 concurrent
- Split View 2 Rooms

Budgets on reference hardware:

- pan/zoom: p95 frame under 16.7 ms during normal view
- Room entry: interactive under 200 ms when cached
- layout worker: under 100 ms for 200 visible nodes
- initial desktop usable: under 2.5 s warm start

### 17.4 Sync spike

Test:

- duplicated operation delivery
- out-of-order pull
- offline edits from two devices
- deleted target conflict
- 24-hour disconnect
- lease expiration during Agent Run
- snapshot compaction while old client reconnects

## 18. Explicitly Deferred Decisions

次は現時点で固定しない。

- CockroachDB adoption and cloud vendor
- NATS or managed pub/sub
- Yjs/CRDT for rich text
- cloud-hosted Agent Runner
- mobile client
- WebGL renderer
- extension/plugin marketplace
- bundled provider binaries
- end-to-end encrypted shared project mode

defer は無視ではない。各項目には採用 trigger と必要な measurement を持たせる。

## 19. Decision Consequences

### Benefits

- subscription-authenticated Agent を local credential のまま使える
- Desktop と Web の FlowFold UI を共有できる
- offline で設計と実装を継続できる
- Synthify の既存 Go/PostgreSQL/Auth/Next.js 資産を活かせる
- provider ごとの approval、usage、session 能力差を正直に扱える
- CockroachDB を必要になった時に導入できる schema discipline を先に持てる

### Costs

- Electron update と security maintenance が必要
- SQLite と cloud DB の二つの persistence model を持つ
- sync protocol と conflict UI が必要
- Codex と Antigravity の adapter behavior は一致しない
- local-only data と shared data の境界を継続的に監査する必要がある

### Rejected shortcuts

- remote Synthify Web を Node-enabled Electron window に読み込む
- Desktop から cloud DB へ直接接続する
- provider token を Hub に預ける
- provider thread を Insightify domain ID として使う
- raw conversation log を merge model にする
- CockroachDB を embedded desktop DB として使う
- brittle CLI text parsing を structured quota API と見なす

## 20. Initial ADR Queue

この文書を親判断とし、実装開始前に次の ADR へ分割する。

1. `0001-electron-desktop-shell.md`
2. `0002-local-sqlite-and-cloud-sync.md`
3. `0003-codex-app-server-provider.md`
4. `0004-antigravity-cli-provider.md`
5. `0005-postgresql-first-cockroach-ready.md`
6. `0006-dom-svg-flowfold-renderer.md`
7. `0007-local-secrets-and-share-policy.md`

ADR は library version の一覧ではなく、変更コストが高い boundary とその理由を固定する。

## 21. Final Recommendation

最初の実装順は次とする。

```text
Electron shell
  -> FlowFold shared packages
  -> SQLite local state
  -> Codex app-server vertical integration
  -> Git worktree + ChangeSet review
  -> Synthify Hub sync/review
  -> Antigravity CLI adapter
  -> branching / lease / multi-agent
  -> CockroachDB decision by measured need
```

Insightify の application center は Desktop に置く。一方、共同作業の center は Synthify Hub に置く。
AI の credential と execution は local、共有すべき intent、decision、operation、evidence は cloud、という境界を崩さない。
