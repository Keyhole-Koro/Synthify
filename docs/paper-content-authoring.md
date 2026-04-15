# Paper Content Authoring

This document defines the current authoring model for paper-in-paper content in this repository.

It is not limited to the landing page.
The landing page is simply the first place where the authoring style is being made explicit.

The goal of this document is to capture the current shape first, then improve it deliberately.

## Scope

This document applies to hand-authored `ContentNode` trees used by paper-in-paper content.

Current known usage:

- `frontend/src/features/landing/landingPaperMap.tsx`

Future paper-in-paper authored content should follow the same rules unless the rules are revised here.

## Goal

Paper content should be authored for readability first.

Authors should be able to read a content definition as content, not as a low-level syntax tree.

This matters for both:

- human editing
- LLM-assisted generation

## Current Model

The current content model is based on `ContentNode` trees from paper-in-paper.

At the authoring level, the main helpers currently used are:

- `t(value)` for text nodes
- `p(...children)` for paragraph nodes
- `prose\`...\`` for paragraph authoring with inline interpolation
- `b(...children)` for bold nodes
- `link(paperId, label)` for paper links
- `section(title, ...children)` for sections
- `list(...items)` for lists
- `table(headers, rows)` for tables
- `card(paperId, title, description)` for card nodes
- `callout(...children)` for callout blocks

This is the current implementation model, not yet the final ideal DSL.

## Current Block-Level Primitives

These are the current top-level content blocks.

- `section(title, ...children)`
- `p(...children)`
- `prose\`...\``
- `list(...items)`
- `table(headers, rows)`
- `card(paperId, title, description)`
- `callout(...children)`

## Current Inline Primitives

These are the current inline-level nodes used inside paragraphs or list items.

- plain text via `t(...)`
- bold via `b(...)`
- paper links via `link(...)`

## Current Paragraph Rule

For new authoring, paragraph-like content should prefer `prose`.

Preferred:

```ts
prose`複数のドキュメントを読み込み、${link('graph', '知識グラフ')}を自動生成します。`
```

Avoid for new content:

```ts
p(
  t('複数のドキュメントを読み込み、'),
  link('graph', '知識グラフ'),
  t('を自動生成します。'),
)
```

Reason:

- the sentence remains readable as a sentence
- inline links are still explicit
- diffs are smaller
- LLM generation is easier to constrain

`p(...)` is still part of the current model, but it should be treated as a lower-level primitive.

## Current List Rule

Lists are currently authored in a lower-level shape than paragraphs.

Current shape:

```ts
list(
  [t('テキスト正規化・チャンク分割')],
  [link('canonicalization', 'エイリアス正規化')],
)
```

This reflects the current implementation, not a polished final form.

Current rule:

- keep list items simple
- prefer one semantic unit per item
- avoid deeply nested inline composition unless needed

## Current Problems

The current model works, but it still has friction.

### Paragraphs

`prose` improves authoring substantially, but still depends on low-level inline helpers like `t`, `b`, and `link`.

### Lists

`list(...ContentNode[][])` is still too close to the raw AST shape.

It exposes implementation structure directly to the author:

- nested arrays
- manual item grouping
- low-level node composition

This is acceptable for now, but it is not considered the ideal authoring surface.

### Helper Quality

Thin wrappers that only rename syntax without reducing complexity are not automatically improvements.

For example, replacing `[t('...')]` with a wrapper that still requires the same mental model is not enough by itself.

## Authoring Principles

Until the DSL is revised, use these principles:

- content readability comes before structural cleverness
- prefer `prose` for any sentence-like content
- use explicit `link(...)` for paper references
- keep list items structurally simple
- do not introduce new helpers unless they reduce authoring friction in a real way

## Non-Goals

This document does not define a full CMS or markdown system.

Not goals:

- a general-purpose markdown parser
- a schema compiler
- a universal content model for the whole app

## Future Improvement Direction

This section is not the current spec.
It is the direction to evaluate next.

### 1. Improve `list(...)` Instead of Adding Thin Wrappers

If list authoring remains awkward, the preferred next step is to make `list(...)` smarter.

Preferred future shape:

```ts
list(
  'テキスト正規化・チャンク分割',
  link('canonicalization', 'エイリアス正規化'),
  prose`${b(t('owner'))} - 全権限・メンバー管理`,
)
```

That would allow the implementation to normalize:

- `string` -> text item
- `ContentNode` -> single-node item
- paragraph-like node -> list item

### 2. Revisit Inline Helpers

If inline authoring still feels noisy, introduce better authoring helpers only when they actually reduce complexity.

Possible examples:

- `strong('text')` instead of `b(t('text'))`
- richer list item normalization

### 3. Keep the DSL Thin

The preferred direction is still a thin, local authoring DSL.

The target is:

- readable by humans
- easy to diff
- easy to generate with an LLM
- close enough to the runtime model to stay understandable

## Decision Summary

Current spec:

- paper-in-paper content is authored as `ContentNode` trees
- `prose` is the preferred paragraph authoring primitive
- `p(...)` remains valid but should be treated as lower-level
- `list(...)` is currently valid but not yet ideal

Current improvement stance:

- document the current shape first
- improve deliberately from there
- optimize for authoring clarity, not abstraction for its own sake
