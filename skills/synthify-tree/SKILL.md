---
name: synthify-tree
description: Turn documents (papers, specs, meeting notes, a repository's docs, a long conversation) into a paper tree, a page of short nested papers where a link in a paper opens its child inside it, published with the self-hosted `artifact` tool. Use when the user asks to organise or structure material so it can be explored, to "make a tree", "map" or "Synthify" it, or to change such a tree.
---

# Building a paper tree

A paper tree is a page where each idea is a short paper. A link in a paper's text opens the
child paper inside it, so the reader starts from one overview and opens only the branches they
need. Every paper must make sense on its own, at every level.

You write the tree as a STIF file. `scripts/build.mjs` checks it and builds a folder with the
viewer, and you publish that folder with the self-hosted `artifact` tool (see the
selfhost-artifact-publish skill).

## Workflow

1. **Read all of the source.** For a long source (more than about 20 pages, or several files),
   first take a short note per section, with where it is (file and lines, or page), and build
   the tree from the notes. Keep such notes outside the folder you publish.
2. **Decide the root:** the one or two sentences a reader should know after reading only the
   root paper. The root paper says them and links to the main topics.
3. **Plan the structure** with "Structure" in `references/authoring.md`: the main topics under
   the root, what goes under each, and where papers should link to each other. The user's
   request decides what the main topics are about ("as a technical spec", "so the decisions and
   open questions stand out"); keep the source's own sections where they fit it.
4. **Write `tree.stif`** in the working folder, following `references/stif.md` for the format and
   `references/authoring.md` for the papers. Images go in an `images/` folder next to it.
5. **Build:** `node <this skill's folder>/scripts/build.mjs tree.stif`. It writes `tree-page/`
   next to the file. Fix every error and read every warning, then build again until it passes.
   `--check` checks without building.
6. **Publish the folder** with the `publish` tool of the self-hosted `artifact` MCP server:
   `publish({ file_path: "<absolute path>/tree-page", description: "<the root's description>" })`,
   or `artifact publish tree-page` in a terminal. The page's title is the root's title. A tool
   that publishes a single HTML page (such as claude.ai's Artifact tool) cannot publish this
   folder. Give the user the URL. A link to `<url>#open=<path>` opens that paper, for example
   `#open=/overview/auth`.

If the self-hosted `artifact` tool is not available, give the user the folder: it is a static
site that any web server can serve (opening `index.html` as a file does not work, because it
fetches `tree.json`).

## Changing a tree

Edit the `.stif`, build into the same folder and publish it again: the URL stays, and the old
version stays available. Keep the paths of papers that stay, so links with `#open=` keep
working. To change a tree published from another computer, `read({ url, path: "tree.stif" })`
gets its STIF.

## Check before publishing

- [ ] The root paper alone tells the reader what the source is about and its main point.
- [ ] Every paper is one idea and makes sense without its parent.
- [ ] Every child is linked from its parent's text, where the reader would want it.
- [ ] Titles are short noun phrases in the source's own terms.
- [ ] Numbers, names and units are exactly as in the source; anything inferred says so.
- [ ] The build passes with no errors, and you read every warning.
