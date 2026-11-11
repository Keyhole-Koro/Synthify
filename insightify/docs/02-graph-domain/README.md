# Graph Domain

Structure Graph と Flow Graph の正本、Revision、ChangeSet を扱う。

予定する主文書:

- `domain-model.md` — Project、Flow、Scope、Node、Port、Edge、Boundary Binding
- `revision-spec.md` — immutable Revision、operation log、materialized view
- `changeset-spec.md` — operation、preview、validation、apply、inverse ChangeSet
- `contract-spec.md` — Node、Port、Edge、Acceptance、Evidence
- [`automatic-generation.md`](automatic-generation.md) — 選択中のAIによる初期Graph生成、安全境界、永続化

Collaboration や Agent の都合を Graph entity 自体へ混ぜず、Anchor を通して関連付ける。
