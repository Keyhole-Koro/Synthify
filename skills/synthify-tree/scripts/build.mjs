#!/usr/bin/env node
// Checks a STIF/1 file and builds the folder to publish: the paper-in-paper viewer, tree.json,
// the images the papers use, and the .stif itself (so the tree can be changed later).
//
//   node build.mjs <tree.stif> [<out-dir>]   build (out-dir defaults to <name>-page next to the file)
//   node build.mjs --check <tree.stif>       check only
//   node build.mjs --serve <tree.stif>       build, then serve the folder on 127.0.0.1 to look at it
//
// No dependencies: Node 18 or later, or Bun.
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { basename, dirname, extname, join, normalize, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const VIEWER = fileURLToPath(new URL('../assets/viewer/', import.meta.url));
const MAX_PAPERS = 1000;
const MAX_CONTENT = 64 * 1024;
const MAX_TOTAL = 8 * 1024 * 1024;
const KNOWN_KEYS = new Set(['title', 'description', 'parent', 'order', 'state', 'scope', 'cross_document', 'merge_key', 'source']);
// Keys whose value is text even when it looks like a number, true or a list ("title: 2024").
const TEXT_KEYS = new Set(['title', 'description', 'parent', 'state', 'scope', 'merge_key', 'type', 'path', 'url', 'repo', 'ref', 'id', 'kind']);
const PATH_RE = /^(\/[a-z0-9]+(?:-[a-z0-9]+)*)+$/;
const IMAGE_TYPES = new Set(['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg', '.avif']);

class Report {
  constructor(file) {
    this.file = file;
    this.errors = [];
    this.warnings = [];
  }
  error(line, message) {
    this.errors.push(`${this.file}:${line}: ${message}`);
  }
  warn(line, message) {
    this.warnings.push(`${this.file}:${line}: warning: ${message}`);
  }
}

// --- Parsing -------------------------------------------------------------------------------

function parseValue(key, raw, line, report) {
  const text = raw.trim();
  if (TEXT_KEYS.has(key) && !text.startsWith('"')) return text;
  if (text.startsWith('"') || text.startsWith('[')) {
    try {
      return JSON.parse(text);
    } catch {
      report.error(line, `cannot read the value ${text} (quote strings like JSON: "…", with \\" inside)`);
      return undefined;
    }
  }
  if (text === 'true' || text === 'false') return text === 'true';
  if (/^-?\d+(\.\d+)?$/.test(text)) return Number(text);
  return text;
}

/** Parses STIF/1 into { attrs, items }. Each item keeps the line numbers its checks report. */
function parseStif(text, report) {
  const lines = text.replace(/^\uFEFF/, '').replace(/\r\n?/g, '\n').split('\n');
  const attrs = {};
  const items = [];
  let i = 0;
  while (i < lines.length && lines[i].trim() === '') i++;
  if (lines[i]?.trim() !== '#STIF/1') {
    report.error(i + 1, 'the file must start with the line #STIF/1');
    return { attrs, items };
  }
  i++;
  let item = null;
  let mode = 'top'; // top | meta | content | css
  let source = null; // the source entry being read, inside "source:"
  for (; i < lines.length; i++) {
    const line = lines[i];
    const n = i + 1;
    if (mode === 'top') {
      if (line.trim() === '') continue;
      if (line.startsWith('@tree')) {
        if (items.length) report.error(n, '@tree must come before the first ::item');
        for (const m of line.matchAll(/([a-z_]+)="((?:[^"\\]|\\.)*)"/g)) attrs[m[1]] = JSON.parse(`"${m[2]}"`);
        continue;
      }
      const m = /^::item\s+(\S+)\s*$/.exec(line);
      if (!m) {
        report.error(n, `expected ::item <path>, found: ${line.slice(0, 60)}`);
        continue;
      }
      item = { path: m[1], line: n, meta: {}, metaLines: {}, sources: [], content: '', contentLine: 0, css: '', cssLine: 0 };
      items.push(item);
      mode = 'meta';
      source = null;
      continue;
    }
    if (/^::end\s*$/.test(line)) {
      if (mode === 'meta') report.error(n, `::item ${item.path} has no --- line before its content`);
      mode = 'top';
      item = null;
      continue;
    }
    if (/^::item\s/.test(line)) {
      report.error(n, `::item ${item.path} (line ${item.line}) has no ::end before this ::item`);
      mode = 'top';
      item = null;
      i--; // read this line again as the start of the next item
      continue;
    }
    if (mode === 'meta') {
      if (line.trim() === '---') {
        mode = 'content';
        item.contentLine = n + 1;
        continue;
      }
      if (line.trim() === '') continue;
      const entry = /^\s*- ([a-z_]+):(.*)$/.exec(line);
      const field = /^\s+([a-z_]+):(.*)$/.exec(line);
      if (entry && source !== null) {
        source = { [entry[1]]: parseValue(entry[1], entry[2], n, report), line: n };
        item.sources.push(source);
        continue;
      }
      if (field && source !== null) {
        if (source.line === undefined) report.error(n, 'start each source with "  - type: …"');
        else source[field[1]] = parseValue(field[1], field[2], n, report);
        continue;
      }
      const kv = /^([a-z_]+):(.*)$/.exec(line);
      if (!kv) {
        report.error(n, `expected "key: value" or --- in ::item ${item.path}, found: ${line.slice(0, 60)}`);
        continue;
      }
      const [, key, raw] = kv;
      if (!KNOWN_KEYS.has(key)) report.warn(n, `unknown key "${key}" (ignored)`);
      if (key in item.meta) report.error(n, `"${key}" is given twice in ::item ${item.path}`);
      item.metaLines[key] = n;
      if (key === 'source') {
        if (raw.trim()) report.error(n, 'source takes a list: put each entry on its own lines, starting with "  - type: …"');
        item.meta.source = true;
        source = {};
        continue;
      }
      source = null;
      item.meta[key] = parseValue(key, raw, n, report);
      continue;
    }
    if (mode === 'content') {
      if (/^::css\s*$/.test(line)) {
        mode = 'css';
        item.cssLine = n + 1;
        continue;
      }
      item.content += `${line}\n`;
      continue;
    }
    item.css += `${line}\n`; // mode === 'css'
  }
  if (item) report.error(item.line, `::item ${item.path} has no ::end`);
  return { attrs, items };
}

// --- Checks --------------------------------------------------------------------------------

/** Width as on screen: CJK and full-width characters count twice. */
const displayWidth = (text) => [...text].reduce((w, ch) => w + (/[\u2E80-\u9FFF\uAC00-\uD7AF\uF900-\uFAFF\uFF00-\uFFEF]/.test(ch) ? 2 : 1), 0);
const isLines = (v) => (Number.isInteger(v) && v > 0) || (Array.isArray(v) && v.length === 2 && v.every((n) => Number.isInteger(n) && n > 0) && v[0] <= v[1]);

const lineOf = (text, index, first) => first + (text.slice(0, index).match(/\n/g)?.length ?? 0);

function checkHtml(item, report, stifDir, images) {
  const html = item.content;
  const at = (index) => lineOf(html, index, item.contentLine);
  const forbidden = [
    [/<script\b/gi, 'scripts do not run in a paper; remove the <script>'],
    [/\son[a-z]+\s*=/gi, 'event handlers (onclick=…) do not run in a paper; remove them'],
    [/<(iframe|object|embed|form|link|meta|base)\b/gi, 'this tag is not allowed in a paper'],
    [/@import|url\(\s*['"]?https?:/gi, 'a paper cannot load remote styles or fonts'],
  ];
  for (const [re, message] of forbidden) for (const m of html.matchAll(re)) report.error(at(m.index), `${message}: ${m[0].trim()}`);
  for (const m of html.matchAll(/<img\b[^>]*?\ssrc\s*=\s*(["'])(.*?)\1/gi)) {
    const src = m[2];
    if (src.startsWith('data:')) continue;
    if (/^[a-z][a-z0-9+.-]*:|^\/\//i.test(src)) {
      report.error(at(m.index), `images from other sites are blocked: save ${src} next to the .stif (e.g. images/…) and use the relative path`);
      continue;
    }
    const file = normalize(src.split(/[?#]/)[0]);
    if (file.startsWith('..') || file.startsWith(sep)) {
      report.error(at(m.index), `image ${src} must be inside the folder of the .stif`);
      continue;
    }
    const abs = join(stifDir, file);
    if (!existsSync(abs) || !statSync(abs).isFile()) report.error(at(m.index), `image ${src} not found (looked for ${abs})`);
    else if (!IMAGE_TYPES.has(extname(file).toLowerCase())) report.error(at(m.index), `image ${src}: use png, jpg, gif, webp, svg or avif`);
    else images.add(file);
  }
}

function checkTree(parsed, report, stifDir) {
  const { items } = parsed;
  const byPath = new Map();
  const images = new Set();
  let total = 0;
  if (items.length === 0) report.error(1, 'the file has no ::item');
  if (items.length > MAX_PAPERS) report.error(1, `${items.length} papers; at most ${MAX_PAPERS}`);
  for (const item of items) {
    if (!PATH_RE.test(item.path)) report.error(item.line, `path ${item.path}: use lowercase a-z, 0-9 and hyphens, like /overview/rate-limits`);
    if (byPath.has(item.path)) report.error(item.line, `path ${item.path} is used twice (also on line ${byPath.get(item.path).line})`);
    byPath.set(item.path, item);
    const { title, description, parent, order } = item.meta;
    if (typeof title !== 'string' || !title.trim()) report.error(item.line, `::item ${item.path} needs a title`);
    else if (displayWidth(title) > 32) report.warn(item.metaLines.title, `title "${title}" is long; a short noun phrase reads better (about 15 characters in Japanese, 4 words in English)`);
    if (description === undefined) report.warn(item.line, `::item ${item.path} has no description (one sentence, shown on its link)`);
    else if (typeof description !== 'string') report.error(item.metaLines.description, 'description must be text');
    if (parent !== undefined && typeof parent !== 'string') report.error(item.metaLines.parent, 'parent must be the path of another ::item');
    if (order !== undefined && typeof order !== 'number') report.error(item.metaLines.order, 'order must be a number');
    if (item.meta.source && item.sources.length === 0) report.error(item.metaLines.source, 'source has no entries');
    for (const s of item.sources) {
      if (!s.type) report.error(s.line, 'each source needs a type (local_file, web, github, document or chat)');
      if (typeof s.path === 'string' && /^(\/|~|[A-Za-z]:[\\/])/.test(s.path)) report.error(s.line, `source path ${s.path} is absolute and would show on the page: write it relative to the repository, or just the file name`);
      if (s.lines !== undefined && !isLines(s.lines)) report.error(s.line, 'lines is a range [first, last] or one line number');
      if (s.page !== undefined && !(isLines(s.page) || (typeof s.page === 'string' && s.page.trim()))) report.error(s.line, 'page is one page (12, or a label such as "iv") or a range [first, last]');
    }
    if (!item.content.trim()) report.warn(item.contentLine, `::item ${item.path} has no content`);
    if (item.content.length > MAX_CONTENT) report.warn(item.contentLine, `::item ${item.path} has ${item.content.length} bytes of content; split it into child papers`);
    total += item.content.length + item.css.length;
    checkHtml(item, report, stifDir, images);
    for (const m of item.css.matchAll(/@import|url\(\s*['"]?https?:/gi)) report.error(lineOf(item.css, m.index, item.cssLine), 'a paper cannot load remote styles or fonts');
  }
  if (total > MAX_TOTAL) report.error(1, `the papers hold ${total} bytes; at most ${MAX_TOTAL}`);

  const roots = items.filter((item) => item.meta.parent === undefined);
  if (roots.length !== 1) {
    report.error(roots[1]?.line ?? 1, roots.length ? `only the root may have no parent, but these have none too: ${roots.slice(1).map((r) => r.path).join(', ')}` : 'one paper must have no parent: the root');
  }
  for (const item of items) {
    const parent = item.meta.parent;
    if (typeof parent === 'string' && !byPath.has(parent)) report.error(item.metaLines.parent, `parent ${parent} is not an ::item in this file`);
    else if (typeof parent === 'string' && !item.path.startsWith(`${parent}/`)) report.warn(item.metaLines.parent, `${item.path} is under ${parent}; a path that starts with ${parent}/ keeps the file easy to read`);
  }
  // Every paper must hang from the root. A missing parent or a second root is reported above.
  const root = roots[0];
  if (root) {
    for (const item of items) {
      const seen = new Set();
      let at = item;
      while (at && at !== root && !seen.has(at.path)) {
        seen.add(at.path);
        at = typeof at.meta.parent === 'string' ? byPath.get(at.meta.parent) : undefined;
      }
      if (at && at !== root) report.error(item.line, `${item.path} does not lead up to the root ${root.path}: its parents form a loop`);
    }
  }
  // Links between papers.
  const children = new Map(items.map((item) => [item.path, []]));
  for (const item of items) if (typeof item.meta.parent === 'string') children.get(item.meta.parent)?.push(item);
  for (const item of items) {
    const linked = new Set();
    for (const m of item.content.matchAll(/data-paper-id\s*=\s*(["'])(.*?)\1/gi)) {
      linked.add(m[2]);
      if (!byPath.has(m[2])) report.error(lineOf(item.content, m.index, item.contentLine), `data-paper-id="${m[2]}" names no ::item`);
    }
    for (const child of children.get(item.path) ?? []) {
      if (!linked.has(child.path)) report.warn(item.contentLine, `${item.path} does not link to its child ${child.path} in its text (it is listed under "Related" instead)`);
    }
  }
  return { byPath, children, root, images };
}

// --- Output --------------------------------------------------------------------------------

const escapeHtml = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);

function sourcesHtml(sources, lang) {
  const items = sources.map((s) => {
    const name = s.title ?? s.path ?? s.url ?? s.id ?? s.type;
    const [first, last] = Array.isArray(s.lines) ? s.lines : [s.lines, s.lines];
    const lines = first === undefined ? '' : first === last ? ` L${first}` : ` L${first}–${last}`;
    const [from, to] = Array.isArray(s.page) ? s.page : [s.page, s.page];
    const page = from === undefined ? '' : from === to ? ` p.${from}` : ` pp.${from}–${to}`;
    const text = escapeHtml(`${name}${lines}${page}`);
    return typeof s.url === 'string' && /^https?:\/\//.test(s.url) ? `<a href="${escapeHtml(s.url)}">${text}</a>` : text;
  });
  const label = lang.startsWith('ja') ? '出典' : 'Sources';
  return `<footer style="margin-top:16px;padding-top:8px;border-top:1px solid rgba(127,127,127,.25);font-size:12px;line-height:1.5;color:var(--muted)">${label}: ${items.join(' · ')}</footer>\n`;
}

// Browsers show a line break between two Japanese (or Chinese) characters as a space, where the
// author only wrapped the line. Join such lines, tags in between kept; <pre> keeps its lines.
const CJK = '[\\u2E80-\\u9FFF\\uF900-\\uFAFF\\uFF00-\\uFFEF]';
const CJK_BREAK = new RegExp(`(${CJK})((?:<[^>]*>)*)[ \\t]*\\n\\s*((?:<[^>]*>)*)(?=${CJK})`, 'g');

function joinCjkLines(html) {
  return html
    .split(/(<pre\b[\s\S]*?<\/pre>)/i)
    .map((part, i) => (i % 2 ? part : part.replace(CJK_BREAK, '$1$2$3')))
    .join('');
}

function sortedChildren(list) {
  if (!list.some((c) => typeof c.meta.order === 'number')) return list;
  return [...list].sort((a, b) => (a.meta.order ?? Infinity) - (b.meta.order ?? Infinity));
}

function toTreeJson(parsed, checked) {
  const { attrs, items } = parsed;
  const { children, root } = checked;
  const lang = attrs.lang || (items.some((item) => /[぀-ヿ]/.test(item.content + item.meta.title)) ? 'ja' : 'en');
  const papers = items.map((item) => ({
    id: item.path,
    parentId: item.meta.parent ?? null,
    childIds: sortedChildren(children.get(item.path)).map((c) => c.path),
    title: item.meta.title.trim(),
    description: (item.meta.description ?? '').trim(),
    content: joinCjkLines(item.content.trim()) + (item.sources.length ? `\n${sourcesHtml(item.sources, lang)}` : '\n'),
    ...(item.css.trim() ? { css: item.css.trim() } : {}),
  }));
  papers.sort((a, b) => (a.id === root.path ? -1 : b.id === root.path ? 1 : 0));
  return { format: 'synthify-tree/1', title: attrs.title || root.meta.title.trim(), lang, rootId: root.path, papers };
}

function depthOf(tree) {
  const parent = new Map(tree.papers.map((p) => [p.id, p.parentId]));
  let max = 0;
  for (const p of tree.papers) {
    let d = 1;
    for (let at = p.parentId; at; at = parent.get(at)) d++;
    max = Math.max(max, d);
  }
  return max;
}

function prepareOutDir(out) {
  if (existsSync(out)) {
    const entries = readdirSync(out);
    const ours = entries.includes('tree.json') && /"format":\s*"synthify-tree\/1"/.test(readFileSync(join(out, 'tree.json'), 'utf8'));
    if (entries.length && !ours) throw new Error(`${out} is not empty and is not a tree built here; choose an empty or new folder`);
    rmSync(out, { recursive: true, force: true });
  }
  mkdirSync(out, { recursive: true });
}

function writeSite(out, stifPath, tree, images) {
  prepareOutDir(out);
  const page = readFileSync(join(VIEWER, 'index.html'), 'utf8')
    .replace(/<html lang="[^"]*">/, `<html lang="${escapeHtml(tree.lang)}">`)
    .replace(/<title>[^<]*<\/title>/, `<title>${escapeHtml(tree.title)}</title>`);
  writeFileSync(join(out, 'index.html'), page);
  for (const file of readdirSync(VIEWER)) if (file !== 'index.html') copyFileSync(join(VIEWER, file), join(out, file));
  writeFileSync(join(out, 'tree.json'), `${JSON.stringify(tree, null, 2)}\n`);
  copyFileSync(stifPath, join(out, 'tree.stif'));
  for (const image of images) {
    mkdirSync(dirname(join(out, image)), { recursive: true });
    copyFileSync(join(dirname(stifPath), image), join(out, image));
  }
}

// --- Main ----------------------------------------------------------------------------------

/**
 * Reads and checks a STIF file's text. `dir` is the folder images are looked up in. Returns the
 * tree.json to write (when there are no errors), the images it uses, and what to report.
 */
export function compileStif(text, { file = 'tree.stif', dir = '.' } = {}) {
  const report = new Report(file);
  const parsed = parseStif(text, report);
  // Check what could be read even when some of it could not, so one run lists every error.
  const checked = parsed.items.length || !report.errors.length ? checkTree(parsed, report, dir) : null;
  const tree = report.errors.length ? null : toTreeJson(parsed, checked);
  return { tree, images: checked?.images ?? new Set(), errors: report.errors, warnings: report.warnings, depth: tree ? depthOf(tree) : 0 };
}

const CONTENT_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.stif': 'text/plain; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
  '.svg': 'image/svg+xml',
  '.avif': 'image/avif',
};

/**
 * Serves a built folder on 127.0.0.1, on a port the system picks (so it never takes one that is
 * in use), to look at the page before publishing. Calls onListening with the URL.
 */
export function serveFolder(out, onListening) {
  const root = resolve(out);
  const server = createServer((req, res) => {
    let path = '';
    try {
      path = decodeURIComponent(new URL(req.url ?? '/', 'http://localhost').pathname);
    } catch {
      // A malformed escape names no file.
    }
    const file = resolve(root, `.${path.endsWith('/') ? `${path}index.html` : path}`);
    if (!file.startsWith(root + sep) || !existsSync(file) || !statSync(file).isFile()) {
      res.writeHead(404, { 'content-type': 'text/plain' }).end('not found');
      return;
    }
    res.writeHead(200, { 'content-type': CONTENT_TYPES[extname(file).toLowerCase()] ?? 'application/octet-stream', 'cache-control': 'no-store' });
    res.end(readFileSync(file));
  });
  server.listen(0, '127.0.0.1', () => onListening(`http://127.0.0.1:${server.address().port}/`));
  return server;
}

function main(argv) {
  const check = argv.includes('--check');
  const serve = argv.includes('--serve');
  const [stifArg, outArg] = argv.filter((a) => !a.startsWith('--'));
  if (!stifArg) {
    console.error('usage: node build.mjs <tree.stif> [<out-dir>] [--serve]\n       node build.mjs --check <tree.stif>');
    return 2;
  }
  const stifPath = resolve(stifArg);
  const { tree, images, errors, warnings, depth } = compileStif(readFileSync(stifPath, 'utf8'), {
    file: relative(process.cwd(), stifPath) || basename(stifPath),
    dir: dirname(stifPath),
  });
  for (const w of warnings) console.error(w);
  for (const e of errors) console.error(e);
  if (errors.length) {
    console.error(`\n${errors.length} error(s): fix them and run the build again.`);
    return 1;
  }
  const count = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;
  const summary = `${count(tree.papers.length, 'paper')}, ${count(depth, 'level')}, ${count(images.size, 'image')}`;
  if (check) {
    console.log(`OK: ${summary}.`);
    return 0;
  }
  const out = resolve(outArg ?? join(dirname(stifPath), `${basename(stifPath, extname(stifPath))}-page`));
  writeSite(out, stifPath, tree, images);
  console.log(`Built ${out}: ${summary}.`);
  console.log(`Publish the folder (artifact publish ${out}). A link to <page url>#open=${tree.papers[1]?.id ?? tree.rootId} opens that paper.`);
  if (serve) serveFolder(out, (url) => console.log(`Serving it at ${url} to look at before publishing. Stop it with Ctrl-C.`));
  return 0;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    process.exitCode = main(process.argv.slice(2));
  } catch (e) {
    console.error(e instanceof Error ? e.message : String(e));
    process.exitCode = 1;
  }
}
