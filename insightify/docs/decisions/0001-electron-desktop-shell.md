# 0001: Electron Desktop Shell

**Status:** Accepted  
**Date:** 2026-08-28

## Context

Insightify は FlowFold の描画だけでなく、Codex app-server、Antigravity CLI、Git worktree、PTY、SQLite、
OS keychain を端末上で監督する必要がある。既存 UI 資産は React/TypeScript である。

## Decision

MVP desktop shell に Electron を使用する。Renderer は sandbox、`nodeIntegration: false`、
`contextIsolation: true` とし、privileged operation は Main または Utility Process のみで実行する。
Preload は versioned typed IPC の最小 surface だけを公開する。

## Alternatives

- Tauri 2: 配布サイズは小さいが、MVP では Rust bridge と process stream 実装が増える。
- Browser + daemon: installation、auth callback、lifecycle、security UX が二製品に分かれる。
- Native Swift/Windows UI: FlowFold の Web 共有が難しく、cross-platform MVP に向かない。

## Consequences

- React UI を Desktop と Hub Web で共有できる。
- Node.js から provider subprocess を直接監督できる。
- Electron/Chromium の更新、code signing、IPC security review が継続的に必要になる。
- Tauri 再評価は実測した memory/startup/update budget が不合格の場合に限る。
