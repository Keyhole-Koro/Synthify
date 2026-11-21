# Writing the papers

The reader sees the root paper first and opens a child by clicking a link in its text. The child
opens inside the parent, next to the papers already open. So each paper is read on its own, and
each link is a promise that the paper behind it answers what the link says.

## Structure

- **The root** says what the source is and its main point in one or two short paragraphs, and
  links to the main topics from that text. It is the summary a busy reader can stop at, not a
  table of contents: give the few facts they must know (the key numbers, the decisions) and
  leave tables, conditions and details to the children.
- **Main topics** are usually 3 to 7 under the root. The user's request decides what they are
  about: a spec by its parts (authentication, errors, limits), meeting notes by outcome
  (decisions, open questions, action items), a paper by question (problem, method, results,
  limits). Make only the topics the source has: a paper without a limitations section gets no
  "limits" paper, and the trade-offs it states go where it discusses them. Within that, keep
  the source's own sections and their names where they fit. Keep what the request does not
  mention as well, after what it asks for; do not drop it.
- **Bibliographic details** (authors, venue, acknowledgements, the reference list) go in one
  paper at the end. Leave out contact details such as email addresses.
- **Depth** follows the source. Make a child when a part of a paper is a separate idea a reader
  may or may not want. A long source usually needs 2 to 4 levels; do not make a paper for a
  single sentence or a single number there.
- **A short source** (a page or less) makes a small tree: the root and one paper per section is
  enough, even when a section is a sentence or two. Do not split it further, and do not pad
  papers to make them longer.
- **One idea per paper.** Split a paper that covers two ideas or runs past about three
  paragraphs. Merge papers that cannot be understood apart.
- **Links across the tree** are welcome: when a paper mentions something another paper explains
  (a sibling, a cousin), link it with `data-paper-id`. The reader can follow ideas, not only
  the hierarchy.

## Title

- A noun phrase in the source's terms, the words a reader would search for: `認証`,
  `エラーハンドリング`, `Rate limits`.
- Short: up to about 15 characters in Japanese or 4 words in English. The build warns past
  about twice that.
- No "について", "の説明", "Overview of", "Introduction to".
- The root names the whole source (`検索 API 仕様`). When the source has no name, name what it
  is about (`検索機能の打ち合わせ`, `API 仕様`).

## Description

One sentence that says what the paper covers, shown on its link card: `Bearer token による認証と、
期限切れの扱いを説明する。` / `How clients authenticate and what happens when a token expires.`
In Japanese, end with 「〜を扱う」 or 「〜を説明する」.

## Content

- 1 to 3 paragraphs (`<p>`). Lead with the point, then the details. Mark the key terms with
  `<strong>`, a few per paper.
- Link each child where the text mentions it, with words that say what the reader will get:
  `失敗は<a data-paper-id="/overview/errors">共通のエラー形式</a>で返す`, not `詳しくは
  <a …>こちら</a>`. A child that is not linked is still listed at the end of the paper under
  "Related", but a link in the text reads better.
- **Tables** for several items that share the same attributes: status codes and their causes,
  limits and their units, owners and their tasks. A single value stays in the sentence. A table
  may gather items the source gives in different sections (every status code it names), with
  each section in the paper's `source`.
- **Lists** for steps and other items without attributes, even when the source writes them as
  sentences. Everything else is prose.
- Keep every number, unit and name exactly as the source has it.
- **Figures** from the source (an architecture diagram, a key plot) when they explain more than
  the text; see "Images" in `stif.md`. Give each a `<figcaption>` with its number and where it
  comes from (「図 1（原論文 p.3）」). Include them when the source's license or the user allows
  it; papers often allow reproduction with attribution.
- Each paper stands alone: name the subject again instead of "it" or "the above", and do not
  rely on the parent having been read.

## Facts

- Only what the source says. Restructure and summarise it; do not add to it. Restating,
  naming and grouping what the source says is not adding: the source's 401, 403 and 400 in a
  table of status codes need no mark.
- Add an inference, a claim the source does not make, only when the reader needs it to
  understand the source, and mark it (「推測:」, "Likely:"). Do not guess at what the source
  leaves out.
- A value you work out from the source shows how: 「上限 10,000 item の 80%（8,000 item）」.
- When a reader would look for something the source does not state (a token's lifetime in a
  spec, an owner or a deadline in meeting notes), say so in one short sentence in the paper
  where they would look: 「有効期限は原文に記載がない。」 Do not repeat it in other papers, and
  do not list everything that is missing.
- Keep the source's uncertainty: an open question stays open, a proposal is not a decision, a
  target (「目標とする」) is not a guarantee.
- When the source contradicts itself (the text gives 41.0 where its table gives 41.8), say so
  in the paper where the reader would look, with both values and where each appears. Do not
  pick one silently.
- Record where each paper comes from with `source` (see `stif.md`). Never invent a path, line,
  page, URL or document.

## Language

Write the papers in the language of the source, unless the user asks for another, and set
`@tree lang` to it. Keep technical terms as the source writes them. When the papers are in
another language than the source (an English paper organised in Japanese):

- Translate the prose. Keep the names of methods, models, datasets and metrics as the source
  writes them (`Multi-Head Attention`, `BLEU`). A general term may be translated, with the
  source's word in parentheses the first time: 残差接続（residual connection）.
- Keep numbers and units as written (`36M`, `28.4 BLEU`). Translating number words is fine
  (4.5 million → 450 万); changing a value is not.

## Writing styles the user may ask for

How to group the papers is under "Structure". These change how the papers are written:

| They ask for | Do |
|---|---|
| Prose (散文で) | Paragraphs instead of lists |
| Definitions (用語解説重視) | A paper per key term, each starting with its definition |
| The source's structure (章構成を維持) | The source's chapters and sections as the papers, in the same order |
| Expert terms (専門用語のまま) | Keep abbreviations and terms unexpanded |
