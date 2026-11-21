# STIF/1, as the build reads it

STIF (Synthify Tree Item Format) is a text file of papers. Each paper is a block with a few
`key: value` lines, then its HTML. The same file can later be imported into Synthify.

## Example

```stif
#STIF/1
@tree lang="ja"

::item /overview
title: "検索 API"
description: "検索 API の認証、エラー、呼び出しの制限を扱う。"
---
<p>検索 API は <a data-paper-id="/overview/auth">Bearer token で認証</a>し、
失敗はすべて<a data-paper-id="/overview/errors">共通のエラー形式</a>で返す。
呼び出しは<a data-paper-id="/overview/rate-limits">クライアントごとに制限</a>される。</p>
::end

::item /overview/auth
title: "認証"
description: "Bearer token による認証と、期限切れの扱いを説明する。"
parent: /overview
source:
  - type: local_file
    path: docs/api.md
    lines: [12, 30]
---
<p>クライアントは <code>Authorization: Bearer &lt;token&gt;</code> を毎回送る。
token が<strong>期限切れ</strong>なら 401 が返り、再認証が要る。</p>
::end
```

## The file

- The first line is `#STIF/1`.
- `@tree` (optional, before the first item) sets `lang` (the language of the papers: `ja`, `en`…)
  and `title` (the page title; the root paper's title by default). Write `lang` when the papers
  are not in English.
- Each paper starts with `::item <path>` and ends with `::end`.
- Inside it: the `key: value` lines, then a line `---`, then the HTML. Optionally `::css` and CSS
  for that paper only, before `::end`.

## Paths

`/overview`, `/overview/auth`, `/overview/auth/token-expiry`: lowercase a–z, 0–9 and hyphens,
in English even when the papers are not. The root's path is one segment naming the whole
(`/overview`, `/api`, `/minutes`), and a paper's path is its parent's path plus one segment.
The path is the paper's id: links and `#open=` use it, so keep it when you change the paper.

## Keys

| Key | Required | Value |
|---|---|---|
| `title` | yes | The paper's title. `"…"` or bare text |
| `description` | recommended | One sentence, shown on the paper's link card |
| `parent` | all but the root | The path of the parent paper. Exactly one paper (the root) has none |
| `order` | no | A number. Children appear in file order; give `order` (10, 20, 30…) to all of a paper's children to set another |
| `source` | recommended | Where the paper's content comes from (below) |

Quote a value with `"…"` when it starts with `"` (inside quotes, write `\"` for a quote).
`state`, `scope`, `cross_document` and `merge_key` are kept for importing into Synthify and do
not change the page.

### `source`

A list, one entry per place. The page shows them under the paper as its sources.

```stif
source:
  - type: local_file
    path: docs/spec.md
    lines: [10, 42]
  - type: local_file
    path: notes.md
    lines: 7
  - type: web
    url: https://example.com/spec
    title: "Example spec"
  - type: document
    title: "2024 annual report"
    page: 12
```

`type` is one of `local_file` (`path`, `lines`), `web` (`url`, `title`), `github` (`url`, `repo`,
`ref`, `path`, `lines`), `document` (`title`, `page` or `lines`) and `chat` (`title`).

- `path` is the file as the reader knows it: relative to the repository root for a file in a
  repository, otherwise its name (`notes.md`). Never an absolute path: the page shows it, and
  the build rejects one.
- `lines` is the range the paper draws on, `[10, 42]`, or one line, `7`.
- Only write places that are in the source material; never make up a path, line or URL.

## HTML

The part after `---` is an HTML fragment (no `<html>` or `<body>`). It is shown in a sandboxed
frame with its own styles for text, tables, code and images, in light and dark mode.

- The styled tags: `p`, `strong`, `em`, `a`, `ul`, `ol`, `li`, `table` (with `thead`, `tbody`,
  `tr`, `th`, `td`), `code`, `pre`, `blockquote`, `figure`, `figcaption`, `img`, `h3`, `h4`.
  Others show without styles. The title is the paper's heading, so do not repeat it as `<h1>`.
- **Links to other papers:** `<a data-paper-id="/overview/auth">…</a>`. A child opens inside the
  paper; any other paper is opened where it is in the tree. The build fails on an unknown path.
- **Links to the web:** `<a href="https://…">`. They open in a new tab.
- **Images:** save the file next to the `.stif` (`images/chart.png`) and use
  `<img src="images/chart.png" alt="…">`. Images from other sites are blocked.
- No `<script>`, event handlers (`onclick=`), `<iframe>`, `<form>`, `<link>` or remote CSS: they
  do not work in a paper, and the build fails on them.
- Escape `<`, `>` and `&` in text as `&lt;`, `&gt;`, `&amp;`.
- Wrap long lines as you like. Browsers show a line break between two Japanese characters as a
  space, so the build joins such lines (not inside `<pre>`).
- Colours in `::css` must work in light and dark mode; prefer none.
