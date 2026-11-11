# 0007: Local Credentials and Explicit Share Policy

**Status:** Accepted  
**Date:** 2026-08-28

## Context

Insightifyはlocal repositoryとsubscription-authenticated Agentを扱いながら、Internet経由でGraphと会話を共有する。
credentialやsource codeを暗黙にHubへ送るとproduct trust boundaryが成立しない。

## Decision

Provider token、Git credential、raw terminal output、raw provider event、repository contentはlocal-onlyをdefaultとする。
credentialはOS keychainに置き、SQLiteにはopaque referenceのみ保存する。HubへはGraph operation、共有Thread、structured run statusを送り、
patch、diff、test evidenceはChangeSet単位のopt-inとする。

## Alternatives

- 全run logをcloud保存: reviewは容易だがsecret/source漏洩面が大きい。
- 完全local-only: team reviewとremote collaborationを実現できない。

## Consequences

- share boundary、redaction、support bundle previewの実装が必要になる。
- Hubはprovider credentialを使った代理実行をしない。
- 将来cloud runnerを追加する場合は別credential/billing/security ADRを要求する。
