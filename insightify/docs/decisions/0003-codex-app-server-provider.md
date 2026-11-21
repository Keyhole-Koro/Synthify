# 0003: Codex App-server as the Primary Agent Provider

**Status:** Accepted  
**Date:** 2026-08-28

## Context

Insightify は ChatGPT managed login と subscription usage を利用しつつ、Thread、Turn、approval、sandbox、streaming を
FlowFold の Node anchored UI に統合する必要がある。

## Decision

Codex integration は `codex app-server` を first-class provider とする。Desktop Runtime Host が子プロセスとして起動し、
default stdio JSONL transport を使用する。起動時に `initialize` / `initialized` handshake を行い、provider-native event を
normalized Agent Event に変換する。実験的 WebSocket transport には依存しない。

Reference: [Official Codex app-server documentation](https://learn.chatgpt.com/docs/app-server)

## Alternatives

- Codex SDK: CI/automation向けで、rich clientのauth/approval/history surfaceとは目的が異なる。
- Codex CLI text scraping: protocol compatibility と approval handling が脆い。
- OpenAI API direct: ChatGPT subscription loginとは異なる billing/auth path になる。

## Consequences

- account、rate limits、thread、turn、approval を構造化して扱える。
- Codex version ごとに protocol schema generation と compatibility test が必要になる。
- OpenAI credential file を直接読み取らず、app-server managed login に任せる。
