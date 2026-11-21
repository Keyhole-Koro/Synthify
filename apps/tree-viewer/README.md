# tree-viewer

The page that shows a paper tree built by the `synthify-tree` skill
([`skills/synthify-tree`](../../skills/synthify-tree/SKILL.md)). It loads the `tree.json` next to
it and renders it with [paper-in-paper](https://www.npmjs.com/package/@keyhole-koro/paper-in-paper).
The open papers are kept in the page's `#open=` fragment, so a link or a reload opens the same
ones; on SelfHostArtifacts the fragment shows in the viewer's address bar. Images published with
the tree are read as `data:` URLs, because a paper's frame loads only `https:`, `data:` and
`blob:` images and the page may be served over plain http.

```bash
bun install
bun run build   # writes skills/synthify-tree/assets/viewer: the skill ships the built viewer, so commit it
bun test src    # the fragment helpers, and the skill's build script (STIF → tree.json)
```

It is not one of the root workspaces: it has its own lockfile, so installing it does not install
the rest of the repository.
