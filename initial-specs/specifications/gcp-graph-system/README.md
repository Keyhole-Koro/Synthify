# GCP Graph System Specification

## Overview

このディレクトリには ActionRev の GCP ベースのグラフ化システム仕様を配置する。
論理レベル（なぜ・何）から物理レベル（どう作るか）の順に階層構造で整理している。

## Document Index

### product/ — なぜ作るか・何を作るか

- [overview.md](product/overview.md)
- [requirements.md](product/requirements.md)
- [roadmap.md](product/roadmap.md)

### domain/ — ドメイン概念・データモデル

- [data-model.md](domain/data-model.md)
- [topic-mapping.md](domain/topic-mapping.md)

### design/ — 機能別設計

- [architecture.md](design/architecture.md)
- [ai-pipeline.md](design/ai-pipeline.md)
- [extraction-strategy.md](design/extraction-strategy.md)
- [normalization-tools.md](design/normalization-tools.md)
- [graph-algorithms.md](design/graph-algorithms.md)
- [frontend.md](design/frontend.md)

### contract/ — API・インターフェース契約

- [api-spec.md](contract/api-spec.md)
- [proto/README.md](contract/proto/README.md)

### implementation/ — 物理実装構造

- [backend-structure.md](implementation/backend-structure.md) - frontend / backend / shared contract を含む実装構造

### quality/ — テスト・評価・運用

- [testing-strategy.md](quality/testing-strategy.md)
- [evaluation-data.md](quality/evaluation-data.md)
- [operations.md](quality/operations.md)

## Document Control

| Item | Value |
| --- | --- |
| Document name | GCP Graph System Specification |
| System name | ActionRev |
| Version | 0.3 |
| Status | Draft |
| Last updated | 2026-03-29 |
| Scope | Initial release and planned extensions |
