# Branching and Merge

複数 Agent Run と代替案を安全に分裂・結合する仕組みを扱う。

予定する主文書:

- `branch-model.md` — fork、base Revision、base Git commit、alternative design
- `concurrency-control.md` — read/write set、optimistic validation、write lease
- `merge-spec.md` — Graph operation merge、Artifact three-way merge、Decision integration
- `conflict-ux.md` — compare、rebase、keep as branch、manual resolution

Session transcript の連結ではなく、ChangeSet、Evidence、Decision、Handoff を merge 対象とする。
