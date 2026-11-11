# 0002: Local SQLite and Domain-operation Sync

**Status:** Accepted  
**Date:** 2026-08-28

## Context

Insightify は offline でも Graph、Thread、Agent Run、ChangeSet を扱い、接続時だけ Synthify Hub と協業する。
Cloud database を直接 Desktop へ公開すると認可、offline、schema migration、conflict の境界が崩れる。

## Decision

Desktop working state は local application-data directory の SQLite/WAL に保存する。Hub との同期は DB replication ではなく、
ULID/UUID、base revision、device sequence、idempotency key を持つ domain operation と outbox/inbox で行う。

## Alternatives

- IndexedDB: privileged runtime、backup、migration、複数 process accessとの統合が弱い。
- Direct PostgreSQL/CockroachDB connection: credential と authorization scope が Desktop に漏れる。
- Repository 内 JSON files: transaction、query、migration、concurrent access が弱い。

## Consequences

- offline-first と crash recovery を実現できる。
- local DB と Hub DB の二つの materialized view、同期 cursor、conflict UI が必要になる。
- SQLite file は network filesystem や repository 内へ置かない。
