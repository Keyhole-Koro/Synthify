# Dependency Architecture — 理想構成

## 目的

`root` `api` `worker` `shared` `web` `log-viewer` の責務と依存方向を整理し、
コードベース全体を「変更しやすく、壊れにくく、生成物がずれにくい」構成にする。

このドキュメントは、現状の混線を前提にした**理想形**と、そこへ寄せるための
段階的な移行方針を定義する。

proto / Buf 設定は `Synthify` 本体 repo の `proto/` に置く。
`SynthifyProto` への切り出しは行わない。

---

## 現状の問題

現状は次の要素が混在している。

- Git submodule として `api` `worker` `shared` `web` `log-viewer` が分離されていた（解消済み）
- ルート `go.mod` が `shared` と `log-viewer` を `replace` で束ねている
- `shared` が共通層である一方で `log-viewer` を import している
- TS の generated proto が `web/src/gen` と `log-viewer/ui/src/gen` に分散している
- `web` と `log-viewer/ui` の依存関係が workspace として定義されず、局所的な `file:` 参照や lockfile ににじんでいる

この結果、次の性質が出ている。

- 共通層 `shared` が純粋な基盤層になっていない
- API 契約 generated code の更新箇所が複数あり、差分がずれる
- 「repo 境界」と「コード上の依存境界」が一致していない
- 小さな変更でも複数モジュールに波及しやすい

---

## 理想構成の原則

理想構成は次の4原則に従う。

1. **契約と実装を分ける**
   `shared` は interface と domain を持つ。具象実装は `api` `worker` `log-viewer` などの外側に置く。
2. **依存は内向きに一方向**
   `shared` は下位アプリや adapter を import しない。
3. **generated code は言語ごとに一箇所**
   Go 用、TypeScript 用で生成先を一本化し、複製を持たない。
4. **アプリ間連携は package / module 境界で明示**
   暗黙の `file:` 参照や lockfile だけに現れる依存を避ける。

---

## 目標ディレクトリ構成

最終的には、論理的には次の責務分離を目指す。

```text
proto/                  # API契約の唯一のソース（Synthify repo内）
buf.yaml
buf.gen.yaml            # Go: packages/shared/gen / TS: packages/proto-ts/gen

packages/shared/
  ├─ domain
  ├─ repository interfaces
  ├─ middleware
  ├─ config
  ├─ joblog            # logging / observability の契約
  └─ gen               # Go generated stubs

api/
  ├─ cmd/
  └─ internal/
      ├─ handler
      └─ service

worker/
  ├─ cmd/
  └─ pkg/worker/
      ├─ agents
      ├─ llm
      ├─ pipeline
      └─ tools

log-viewer/
  ├─ go/               # job log persistence / query adapter
  └─ ui/               # log viewer frontend

web/
  └─ product frontend

packages/
  └─ proto-ts/         # TS generated client/types
```

これは「物理ディレクトリを今日すぐ全部変える」という意味ではなく、
**責務の置き場所としての理想形**を示している。

`proto/` は `Synthify` 本体 repo が所有する。

---

## 依存グラフ

### 理想の依存方向

```text
api ----------> shared
worker -------> shared
log-viewer ---> shared

web ----------> packages/proto-ts
log-viewer/ui -> packages/proto-ts

api ----------> log-viewer/go        # job log adapter を使う場合のみ
worker -------> log-viewer/go        # job log adapter を使う場合のみ
```

### 禁止したい依存方向

```text
shared ------X-> log-viewer
shared ------X-> api
shared ------X-> worker
web ---------X-> log-viewer/ui/src/gen
log-viewer/ui X-> 独自 generated proto コピー
```

---

## 各モジュールの責務

### `proto/`

- Connect / gRPC / message schema の唯一の定義元
- API surface の変更はまずここから始める
- `buf.yaml` / `buf.gen.yaml` で Go は `packages/shared/gen` へ、TS は `packages/proto-ts/gen` へ生成する

### `packages/shared`

`shared` は「共通処理」ではなく、**共通契約層**として扱う。

置いてよいもの:

- `domain`
- `repository` interface
- `middleware`
- `config`
- `util`
- `jobstatus` のような共通通知 abstraction
- `joblog` のような logging / observability contract
- Go generated proto / connect stub

置かないもの:

- UI
- DB 永続化の具象実装に強く依存するコード
- `log-viewer` の package 型
- `api` や `worker` の都合でしか使わない orchestration 実装

### `api`

- HTTP / Connect エントリポイント
- auth / authz
- request / response mapping
- application service orchestration
- worker dispatch

`api` は `shared` の interface に依存し、具象 store や logger 実装は composition root で差し込む。

### `worker`

- ジョブ実行
- LLM / tool orchestration
- tree mutation
- evaluation

`worker` も `shared` の契約だけを見て動くようにする。

### `log-viewer`

`log-viewer` は 2 つに分けて考える。

- `log-viewer/go`
  job log の保存・検索・正規化を担う adapter
- `log-viewer/ui`
  log 閲覧 UI

重要なのは、`log-viewer` が **logging 契約そのものの所有者ではない** こと。
契約は `shared/joblog` に置き、`log-viewer` はその consumer / implementation とする。

### `web`

- プロダクト本体 UI
- domain の TS 直持ちは避け、API 契約 generated client / API wrapper を通す
- `log-viewer/ui` を再利用する場合も package dependency として明示する

---

## Logging / Observability の理想配置

現在の一番大きいねじれは、`shared` が `log-viewer` の `Logger` / `Event` を参照している点にある。

これを次のように直す。

### 新設: `shared/joblog`

`shared/joblog` に最小契約だけを置く。

```go
package joblog

type Level string

const (
    INFO  Level = "INFO"
    WARN  Level = "WARN"
    ERROR Level = "ERROR"
)

type Event struct {
    JobID       string
    WorkspaceID string
    DocumentID  string
    Level       Level
    Event       string
    Message     string
    Detail      map[string]any
}

type Logger interface {
    Log(ctx context.Context, e Event)
}
```

加えて、次もここに置く。

- `WithLogger`
- `FromContext`
- `NoopLogger`

### 依存の変化

変更前:

```text
shared -> log-viewer
api    -> log-viewer
worker -> log-viewer
```

変更後:

```text
shared -> shared/joblog
api    -> shared/joblog
worker -> shared/joblog
log-viewer/go -> shared/joblog
```

### adapter の責務

DB 保存や検索は `log-viewer/go` または `shared/repository/postgres` 内の adapter が担う。

つまり:

- `shared/joblog` は contract
- `postgres DBLogger` は implementation
- `api` / `worker` は contract を使うだけ

この形にすると、将来 `stdout logger` `test logger` `OpenTelemetry bridge` を足しても
共通層は汚れない。

---

---

## Proto / Generated Code の理想配置

現状は TS generated code が `web` と `log-viewer/ui` に分散しやすい。
これは drift の温床になるので、次のどちらかに統一する。

### 採用済み: `packages/proto-ts`

```text
packages/
  proto-ts/
    gen/...
```

`buf generate` (`buf.gen.yaml`) はここだけへ TS を出力する。

利用側:

- `web` は `packages/proto-ts` を import
- `log-viewer/ui` も `packages/proto-ts` を import

利点:

- generated code 更新箇所が一箇所
- UI が増えても再利用できる
- `web` と `log-viewer/ui` で契約差分が起きない

### 代替案: 複数出力を正式管理

どうしても package 化しない場合は、`SynthifyProto` 側の `buf.gen.ts.yaml` に
`web` と `log-viewer/ui` の両方を
**正式に**出力先として登録する。

ただしこの案は、生成物の複製を許すため、長期的には推奨しない。

---

## Frontend Workspace の理想配置

`web` と `log-viewer/ui` は独立 package として workspace 管理する。

```text
web/
log-viewer/ui/
packages/proto-ts/
package.json            # workspace root
```

理想は次の状態。

- 依存は `package.json` に明示される
- lockfile はそれを反映するだけ
- `file:` の偶発参照ではなく workspace resolution を使う

この構成なら、

- `web` から `log-viewer/ui` を部品利用する
- `web` と `log-viewer/ui` が同じ generated client を使う

を自然に管理できる。

---

## Repo 戦略

monorepo として一本化済み。submodule は廃止。`proto/` は本 repo が管理する。

---

## 段階的な移行計画

### Phase 1: Logging 契約の切り出し — **完了**

- `packages/shared/joblog/` に契約を移動
- `shared/go.mod` から `SynthifyLogViewer` 依存を除去済み

### Phase 2: Generated TS の一本化 — **完了**

- `packages/proto-ts/gen/` に一本化
- `buf.gen.yaml` が Go / TS 両方の出力先を管理
- `web` / `log-viewer/ui` ともに `@synthify/proto-ts` を参照

### Phase 3: Frontend workspace 化 — **完了**

- ルート `package.json` に `workspaces: [apps/web, apps/log-viewer/ui, packages/proto-ts]`
- 依存は manifest で管理されている

### Phase 4: Repo 境界の整理 — **完了**

- submodule を全廃し `apps/` / `packages/` 配下の通常ディレクトリとして管理
- モジュール名を `github.com/synthify/backend/*` に統一
- `.gitmodules` 廃止

---

## この構成で得られる効果

- `shared` が本当に再利用可能な基盤層になる
- logging / observability の差し替えが容易になる
- proto 契約の drift が減る
- フロントエンドの依存が package manager で追跡可能になる
- API 契約変更の影響範囲が読みやすくなる
- 「どこが契約で、どこが実装か」が明確になる

---

## 非目標

このドキュメントは以下を直接決めない。

- 各 RPC の詳細仕様
- DB テーブル設計の最終形
- log viewer UI の具体的な見た目
- LLM worker のアルゴリズム

それらは別ドキュメントで扱う。

---

## 結論

最優先で直すべきなのは、`shared` が `log-viewer` に依存している逆流である。

そこを `shared/joblog` の導入で切り離し、その後に

すべての Phase が完了し、理想構成に到達した。
