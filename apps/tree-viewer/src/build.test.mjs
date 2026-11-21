// The skill's build script (skills/synthify-tree/scripts/build.mjs): what it accepts, what it
// turns into tree.json, and the errors an agent gets back.
import { describe, expect, test } from 'bun:test';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { compileStif } from '../../../skills/synthify-tree/scripts/build.mjs';

const GOOD = `#STIF/1
@tree title="検索 API"

::item /overview
title: "検索 API 仕様"
description: "要点をまとめる。"
---
<p><a data-paper-id="/overview/auth">認証</a>と<a data-paper-id="/overview/errors">エラー</a>。</p>
<img src="images/fig.png" alt="">
::end

::item /overview/auth
title: 認証
description: 認証を扱う。
parent: /overview
order: 20
source:
  - type: local_file
    path: docs/api.md
    lines: [3, 5]
  - type: web
    url: https://example.com/spec
    title: "Spec <v2>"
---
<p>Bearer token。</p>
::css
p { margin: 0; }
::end

::item /overview/errors
title: 2024
description: "エラーを扱う。"
parent: /overview
order: 10
---
<p>401 / 403。</p>
::end
`;

function dirWithImage() {
  const dir = mkdtempSync(join(tmpdir(), 'stif-'));
  mkdirSync(join(dir, 'images'));
  writeFileSync(join(dir, 'images/fig.png'), 'png');
  return dir;
}

describe('a good file', () => {
  const { tree, images, errors, warnings, depth } = compileStif(GOOD, { dir: dirWithImage() });

  test('compiles with no errors or warnings', () => {
    expect(errors).toEqual([]);
    expect(warnings).toEqual([]);
    expect(depth).toBe(2);
    expect([...images]).toEqual(['images/fig.png']);
  });

  test('becomes tree.json: the root first, children in `order`, the title from @tree', () => {
    expect(tree).toMatchObject({ format: 'synthify-tree/1', title: '検索 API', lang: 'ja', rootId: '/overview' });
    expect(tree.papers.map((p) => p.id)).toEqual(['/overview', '/overview/auth', '/overview/errors']);
    expect(tree.papers[0].childIds).toEqual(['/overview/errors', '/overview/auth']);
    expect(tree.papers[1]).toMatchObject({ parentId: '/overview', css: 'p { margin: 0; }' });
  });

  test('keeps text values as text', () => {
    expect(tree.papers[2].title).toBe('2024');
  });

  test('shows the sources under the paper, escaped', () => {
    const content = tree.papers[1].content;
    expect(content).toContain('出典: docs/api.md L3–5 · <a href="https://example.com/spec">Spec &lt;v2&gt;</a>');
    expect(tree.papers[2].content).not.toContain('<footer');
  });
});

test('a source may name one line, and an equal range shows as one line', () => {
  const { tree, errors } = compileStif('#STIF/1\n::item /root\ntitle: R\ndescription: d\nsource:\n  - type: local_file\n    path: a.md\n    lines: 7\n  - type: local_file\n    path: b.md\n    lines: [9, 9]\n---\n<p>x</p>\n::end\n');
  expect(errors).toEqual([]);
  expect(tree.papers[0].content).toContain('Sources: a.md L7 · b.md L9</footer>');
});

test('Japanese lines wrapped in the file are joined, as browsers would show the break as a space', () => {
  const { tree } = compileStif('#STIF/1\n::item /r\ntitle: R\ndescription: d\n---\n<p>この API は<a data-paper-id="/r">認証</a>し、\n失敗は<a data-paper-id="/r">エラー形式</a>\n  で返す。API は\nBearer token を使う。</p>\n<pre>一行目\n二行目</pre>\n::end\n');
  expect(tree.papers[0].content).toBe('<p>この API は<a data-paper-id="/r">認証</a>し、失敗は<a data-paper-id="/r">エラー形式</a>で返す。API は\nBearer token を使う。</p>\n<pre>一行目\n二行目</pre>\n');
});

test('English papers get the English label and lang', () => {
  const { tree } = compileStif('#STIF/1\n::item /root\ntitle: Root\ndescription: x\nsource:\n- type: chat\n  title: Notes\n---\n<p>Plain text.</p>\n::end\n');
  expect(tree.lang).toBe('en');
  expect(tree.papers[0].content).toContain('Sources: Notes');
});

describe('errors', () => {
  const errorsOf = (text) => compileStif(text, { file: 't.stif', dir: dirWithImage() }).errors;
  const item = (path, meta, html = '<p>x</p>') => `::item ${path}\ntitle: T\ndescription: d\n${meta}---\n${html}\n::end\n`;

  test('a file without the header', () => {
    expect(errorsOf('::item /a\n')).toEqual(['t.stif:1: the file must start with the line #STIF/1']);
  });

  test('the tree: parents, roots, loops, duplicate and badly formed paths', () => {
    const errors = errorsOf(
      `#STIF/1\n${item('/root', '')}${item('/root/a', 'parent: /missing\n')}${item('/other', '')}${item('/b', 'parent: /c\n')}${item('/c', 'parent: /b\n')}${item('/root', 'parent: /root\n')}${item('/Bad_Path', 'parent: /root\n')}`,
    );
    expect(errors.join('\n')).toContain('parent /missing is not an ::item in this file');
    expect(errors.join('\n')).toContain('only the root may have no parent, but these have none too: /other');
    expect(errors.join('\n')).toContain('/b does not lead up to the root /root: its parents form a loop');
    expect(errors.join('\n')).toContain('path /root is used twice (also on line 2)');
    expect(errors.join('\n')).toContain('path /Bad_Path: use lowercase');
  });

  test('HTML a paper cannot use, with the line it is on', () => {
    const errors = errorsOf(
      `#STIF/1\n${item('/root', '', '<p>ok</p>\n<script>x()</script>\n<p onclick="x()">y</p>\n<img src="https://example.com/a.png"><img src="../a.png"><img src="images/none.png">\n<a data-paper-id="/nope">z</a>\n<iframe src="x"></iframe>\n<style>@import url(https://f.example/x.css)</style>')}`,
    );
    expect(errors).toEqual([
      't.stif:7: scripts do not run in a paper; remove the <script>: <script',
      't.stif:8: event handlers (onclick=…) do not run in a paper; remove them: onclick=',
      't.stif:11: this tag is not allowed in a paper: <iframe',
      't.stif:12: a paper cannot load remote styles or fonts: @import',
      't.stif:12: a paper cannot load remote styles or fonts: url(https:',
      't.stif:9: images from other sites are blocked: save https://example.com/a.png next to the .stif (e.g. images/…) and use the relative path',
      't.stif:9: image ../a.png must be inside the folder of the .stif',
      expect.stringMatching(/^t\.stif:9: image images\/none\.png not found/),
      't.stif:10: data-paper-id="/nope" names no ::item',
    ]);
  });

  test('a missing ::end is reported, and the next item is still read', () => {
    const errors = errorsOf(`#STIF/1\n::item /root\ntitle: R\n---\n<p><a data-paper-id="/root/a">a</a></p>\n${item('/root/a', 'parent: /root\n')}`);
    expect(errors).toEqual(['t.stif:6: ::item /root (line 2) has no ::end before this ::item']);
  });

  test('sources with an absolute path or bad lines', () => {
    const errors = errorsOf(`#STIF/1\n::item /root\ntitle: R\nsource:\n  - type: local_file\n    path: /home/me/notes.md\n  - type: local_file\n    path: notes.md\n    lines: [5, 2]\n---\n<p>x</p>\n::end\n`);
    expect(errors).toEqual([
      't.stif:5: source path /home/me/notes.md is absolute and would show on the page: write it relative to the repository, or just the file name',
      't.stif:7: lines is a range [first, last] or one line number',
    ]);
  });

  test('values and sources that cannot be read', () => {
    const errors = errorsOf(`#STIF/1\n::item /root\ntitle: "unclosed\nsource:\n    path: x\n---\n<p>x</p>\n::end\n`);
    expect(errors).toEqual([
      't.stif:3: cannot read the value "unclosed (quote strings like JSON: "…", with \\" inside)',
      't.stif:5: start each source with "  - type: …"',
      't.stif:2: ::item /root needs a title',
      't.stif:4: source has no entries',
    ]);
  });
});

test('warnings: no description, a child not linked from the text, a long title', () => {
  const { errors, warnings } = compileStif(
    '#STIF/1\n::item /root\ntitle: R\n---\n<p>x</p>\n::end\n::item /root/a\ntitle: An unusually long title that goes on\ndescription: d\nparent: /root\n---\n<p><a data-paper-id="/root/a/b">b</a></p>\n::end\n::item /root/a/b\ntitle: 検索結果の並び順とページング方式の検討\ndescription: d\nparent: /root/a\n---\n<p>x</p>\n::end\n',
    { file: 't.stif' },
  );
  expect(errors).toEqual([]);
  const long = 'is long; a short noun phrase reads better (about 15 characters in Japanese, 4 words in English)';
  expect(warnings).toEqual([
    't.stif:2: warning: ::item /root has no description (one sentence, shown on its link)',
    `t.stif:8: warning: title "An unusually long title that goes on" ${long}`,
    `t.stif:15: warning: title "検索結果の並び順とページング方式の検討" ${long}`,
    't.stif:5: warning: /root does not link to its child /root/a in its text (it is listed under "Related" instead)',
  ]);
});

test('titles of about 15 Japanese characters or 4 English words pass', () => {
  const { warnings } = compileStif('#STIF/1\n::item /root\ntitle: SLO・容量制限・運用ルール\ndescription: d\n---\n<p><a data-paper-id="/root/a">a</a></p>\n::end\n::item /root/a\ntitle: How clients authenticate\ndescription: d\nparent: /root\n---\n<p>x</p>\n::end\n');
  expect(warnings).toEqual([]);
});
