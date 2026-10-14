# 検証フェーズ一覧

最終確認日: 2026-09-24

この表は、現在のワークツリーに対する検証結果を記録する。ファイル数は、各行の対象言語に該当する実装ファイルを数える。テストファイル、生成コード（`sqlcgen`、`*.generated.*`、`src/generated`、`routeTree.gen.ts`、`next-env.d.ts`）、ビルド出力は除外する。

## 結果

| フェーズ | 対象ディレクトリ | 言語 | 実装ファイル数 | 検証結果 |
| --- | --- | --- | ---: | --- |
| Phase 1 | `internal/platform/` | Go | 21 | `go vet` / `go test` PASS |
| Phase 2 | `packages/*` と `apps/desktop/` | TypeScript | 26 | 全 package と desktop の typecheck / test PASS（テストなし package は許容） |
| Phase 3 | `apps/eval/` と `apps/monitor/`（`ui/`除外） | Go | 16 | `go vet` / `go test` PASS |
| Phase 4 | `apps/monitor/ui/` | TypeScript | 40 | test 2 files / 11 tests、`next build` PASS |
| Phase 5 | `apps/api/` | Go | 68 | `go vet` / `go test -short` PASS |
| Phase 6 | `apps/worker/` | Go | 87 | `go vet` / `go test -short` PASS |
| Phase 7 | `apps/web/src/{config,shared,lib,client}/` | TypeScript | 20 | typecheck PASS、対象 test 5 files / 28 tests PASS |
| Phase 8 | `apps/web/src/features/{auth,billing,documents,jobs,sharing,landing}/` | TypeScript | 33 | typecheck PASS、対象 test 4 files / 15 tests PASS |
| Phase 9 | `apps/web/src/features/{tree,paperMap,workspaceChat,workspaces}/` | TypeScript | 40 | typecheck PASS、対象 test 10 files / 30 tests PASS |

## 実行コマンド

### Go

```sh
# Phase 1
go vet ./internal/platform/...
go test -count=1 ./internal/platform/...

# Phase 3
go vet ./apps/eval/...
go test -count=1 ./apps/eval/...
(cd apps/monitor && go vet ./...)
(cd apps/monitor && go test -count=1 ./...)

# Phase 5
go vet ./apps/api/...
go test -short -count=1 ./apps/api/...

# Phase 6
go vet ./apps/worker/...
go test -short -count=1 ./apps/worker/...
```

### TypeScript

```sh
# Phase 2: packages/*
for package_dir in packages/*; do
  bun --cwd "$package_dir" typecheck
done
bun --cwd packages/agent-antigravity-cli test -- --passWithNoTests
bun --cwd packages/agent-codex test -- --passWithNoTests
bun --cwd packages/agent-runtime test -- --passWithNoTests
bun --cwd packages/desktop-bridge test
bun --cwd packages/graph-domain test -- --passWithNoTests

# Phase 2: desktop
bun run desktop:typecheck
bun run desktop:test

# Phase 4
bun --cwd apps/monitor/ui test
bun --cwd apps/monitor/ui build

# Phase 7
bun run web:typecheck
bun --cwd apps/web test -- src/config src/shared src/lib src/client

# Phase 8
bun run web:typecheck
bun --cwd apps/web test -- \
  src/features/auth src/features/billing src/features/documents \
  src/features/jobs src/features/sharing src/features/landing

# Phase 9
bun run web:typecheck
bun --cwd apps/web test -- \
  src/features/tree src/features/paperMap \
  src/features/workspaceChat src/features/workspaces
```

## Web テストの集計

Phase 7〜9 はフェーズ単位で対象を分けて実行する。`bun run web:test` は `apps/web` 全体の集計であり、Phase 9単独の結果として記録しない。

| 集計範囲 | test files | tests |
| --- | ---: | ---: |
| Phase 7 | 5 | 28 |
| Phase 8 | 4 | 15 |
| Phase 9 | 10 | 30 |
| `apps/web` 全体（`bun run web:test`） | 19 | 73 |

`apps/web` の Phase 8 に含めるディレクトリ名は `docs` ではなく、実際のパスに合わせて `documents` とする。

## 注意事項

- `apps/monitor/` は独立した Go module のため、ルートからではなく `apps/monitor/` 内で検証する。
- `apps/monitor/ui/` の30件という旧集計は、UI の一部だけを数えた値であり、API route を含む対象ディレクトリ全体の件数ではない。
- `apps/api/` の70件という旧集計は、現在の生成コード除外ルールでは再現できない。現行値は68件である。
- テストなし package は品質失敗ではないが、通常の `vitest run` は exit 1 になるため、Phase 2では `--passWithNoTests` を明示する。
