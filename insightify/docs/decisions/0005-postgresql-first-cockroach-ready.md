# 0005: PostgreSQL First, Cockroach-ready Collaboration Schema

**Status:** Accepted  
**Date:** 2026-08-28

## Context

Synthify は既に PostgreSQL、pgx、sqlc、migration、repository boundary を持つ。CockroachDB は multi-region と
failure survival に強いが、SERIALIZABLE retry と PostgreSQL compatibility差を導入する。

## Decision

MVP Hub は既存 PostgreSQL を使う。新規 collaboration schema は UUID/ULID、idempotent transaction、transactional outbox、
compare-and-swap revisionを使い、advisory lock、LISTEN/NOTIFY、extensionをdomain correctnessに要求しない。

CockroachDB は複数region配置、region failure SLO、data localityのうち複数が実要件になった時点でspikeとADR再採決を行う。

## Alternatives

- CockroachDB from day one: global-readyだがFlowFold MVPと同時にmigration riskを負う。
- Firestore as graph source of truth: realtimeは容易だがsemantic transactionとoperation historyに合わない。

## Consequences

- 現行Synthify資産を再利用できる。
- Cockroach compatibility CIを独立導入できる。
- multi-region requirement成立時には実DB migrationが必要になる。
