# Insightify

Insightify は、ソフトウェアを抽象から具体へ辿れる再帰的な Flow として設計し、Flow 上の任意の場所で
人間と AI が共同作業するための開発環境である。

中核の再帰キャンバス方式を **FlowFold** と呼ぶ。Compound Node は子要素を無限に展開する container ではなく、
独立した Scope / Room へ入るための Portal として扱う。

## Documentation

設計文書は依存順に配置する。番号は実装優先度ではなく、概念を読む順序を表す。

| Directory | Scope |
|---|---|
| [`00-overview`](docs/00-overview/) | プロダクト全体、設計原則、用語、全体アーキテクチャ |
| [`01-flowfold`](docs/01-flowfold/) | Portal、Room、Semantic Zoom、layout、操作仕様 |
| [`02-graph-domain`](docs/02-graph-domain/) | Scope、Node、Port、Edge、Revision、ChangeSet |
| [`03-collaboration`](docs/03-collaboration/) | Anchor、Thread、ThreadMemory、Context Compiler |
| [`04-agent-runtime`](docs/04-agent-runtime/) | Agent Run、Local Runner、tool policy、worktree |
| [`05-branching`](docs/05-branching/) | Branch、read/write set、lease、conflict、merge |
| [`06-mvp`](docs/06-mvp/) | MVP、prototype、demo、評価計画 |
| [`decisions`](docs/decisions/) | 個別の Architecture Decision Record |

最初に読む文書:

- [Insightify System Design](docs/00-overview/system-design.md)
- [Technology Stack and Runtime Architecture](docs/00-overview/technology-stack.md)
- [FlowFold Interaction Specification](docs/01-flowfold/interaction-spec.md)

## Naming

- **Insightify:** プロダクト全体
- **FlowFold:** 再帰的な Flow UI・構造モデル
- **Room:** Scope に入ったときの作業空間
- **Thread:** 人間と AI の継続的な対話
- **Agent Run:** AI が tool を使って行う一回の作業
- **ChangeSet:** Graph と Artifact に対する原子的な変更提案

現行 repository、package、proto namespace に残る `Synthify` の rename は、互換性と移行順序を定める
migration plan を作成してから行う。
