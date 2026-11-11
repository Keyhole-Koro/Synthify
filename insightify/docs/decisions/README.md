# Architecture Decision Records

個別の設計選択と、その理由、代替案、結果を Architecture Decision Record として保存する。

ファイル名:

```text
NNNN-short-decision-title.md
```

初期 template:

```markdown
# NNNN: Decision title

**Status:** Proposed | Accepted | Superseded
**Date:** YYYY-MM-DD

## Context

## Decision

## Alternatives

## Consequences
```

親 Design Doc の大きな方向性は `00-overview` に残し、実装時に再検討しうる局所判断をここへ分離する。

## Accepted Decisions

1. [Electron Desktop Shell](0001-electron-desktop-shell.md)
2. [Local SQLite and Domain-operation Sync](0002-local-sqlite-and-cloud-sync.md)
3. [Codex App-server as the Primary Agent Provider](0003-codex-app-server-provider.md)
4. [Antigravity CLI for Subscription-oriented Runs](0004-antigravity-cli-provider.md)
5. [PostgreSQL First, Cockroach-ready Collaboration Schema](0005-postgresql-first-cockroach-ready.md)
6. [DOM and SVG FlowFold Renderer](0006-dom-svg-flowfold-renderer.md)
7. [Local Credentials and Explicit Share Policy](0007-local-secrets-and-share-policy.md)
