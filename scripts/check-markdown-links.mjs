#!/usr/bin/env node
// Checks the relative links in the repository's Markdown files, offline: a
// link to a file or directory must point at one that git tracks, and a link to
// a heading (#fragment) in a Markdown file must name a heading it has. Links
// with a scheme (https:, mailto:) are not fetched or checked.
//
// Usage: node scripts/check-markdown-links.mjs [file.md ...]
// Without arguments it checks every tracked *.md file. It prints each broken
// link as file:line and exits 1 if there is one.
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { posix } from 'node:path';

const root = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();

// Tracked paths only, so a file that exists on one machine but is not
// committed cannot make a link look fine. A submodule is listed as its path;
// GitHub cannot show the files inside it from this repository.
const files = new Set(
  execFileSync('git', ['ls-files', '-z'], { cwd: root, encoding: 'utf8', maxBuffer: 64 << 20 })
    .split('\0')
    .filter(Boolean),
);
const dirs = new Set(['']);
for (const file of files) {
  for (let dir = posix.dirname(file); dir !== '.' && !dirs.has(dir); dir = posix.dirname(dir)) dirs.add(dir);
}

const read = (file) => readFileSync(posix.join(root, file), 'utf8');

// Fenced code, inline code and HTML comments become spaces, so that links in
// them are not read and the line numbers of the rest stay right.
function prose(text) {
  const lines = text.split('\n');
  let fence = null;
  for (let i = 0; i < lines.length; i++) {
    if (fence) {
      if (fence.test(lines[i])) fence = null;
      lines[i] = '';
      continue;
    }
    const open = /^ {0,3}(`{3,}|~{3,})/.exec(lines[i]);
    if (open) {
      fence = new RegExp(`^ {0,3}${open[1][0]}{${open[1].length},}\\s*$`);
      lines[i] = '';
      continue;
    }
    lines[i] = lines[i].replace(/(`+)(?:[^`]|(?!\1)`+)*?\1(?!`)/g, (span) => ' '.repeat(span.length));
  }
  return lines.join('\n').replace(/<!--[\s\S]*?-->/g, (comment) => comment.replace(/[^\n]/g, ' '));
}

// The ids GitHub gives headings: the text in lower case, punctuation and
// symbols dropped, spaces to hyphens, and -1, -2, ... on repeats.
function headingIds(text) {
  const ids = new Set();
  const counts = new Map();
  const add = (heading) => {
    const plain = heading
      .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
      .replace(/<[^>]+>/g, '')
      .trim()
      .toLowerCase();
    const base = plain.replace(/[^\p{L}\p{M}\p{N}\p{Pc} -]/gu, '').replace(/ /g, '-');
    let id = base;
    let n = counts.get(base) ?? 0;
    while (ids.has(id)) id = `${base}-${++n}`;
    counts.set(base, n);
    ids.add(id);
  };
  // Inline code keeps its text in a heading, so read headings from the text
  // with only fenced code and comments blanked.
  const lines = text.split('\n');
  const blanked = prose(text).split('\n');
  for (let i = 0; i < lines.length; i++) {
    if (blanked[i].trim() === '' && lines[i].trim() !== '') continue;
    const atx = /^ {0,3}#{1,6}(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$/.exec(lines[i]);
    if (atx) {
      add((atx[1] ?? '').replace(/`/g, ''));
    } else if (i > 0 && /^ {0,3}(=+|-+)[ \t]*$/.test(lines[i]) && /\S/.test(blanked[i - 1]) && !/^ {0,3}([-*+]|\d+[.)])\s|^ {0,3}#/.test(lines[i - 1])) {
      add(lines[i - 1].replace(/`/g, ''));
    }
  }
  for (const anchor of text.matchAll(/<a\s[^>]*?(?:name|id)\s*=\s*["']([^"']+)["']/gi)) ids.add(anchor[1]);
  return ids;
}

const idCache = new Map();
function idsOf(file, text) {
  if (!idCache.has(file)) idCache.set(file, headingIds(text ?? read(file)));
  return idCache.get(file);
}

function* links(text) {
  const lines = prose(text).split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    // [text](destination "title") and ![alt](destination)
    for (const m of line.matchAll(/\]\(\s*(<[^<>]*>|(?:[^\s()]|\([^\s()]*\))+)(?:\s+(?:"[^"]*"|'[^']*'|\([^()]*\)))?\s*\)/g)) {
      yield { line: i + 1, target: m[1] };
    }
    // [label]: destination (not footnotes, [^1]: ...)
    const definition = /^ {0,3}\[([^\]^][^\]]*)\]:\s*(<[^<>]*>|\S+)/.exec(line);
    if (definition) yield { line: i + 1, target: definition[2] };
    // <a href="..."> and <img src="..."> written as HTML
    for (const m of line.matchAll(/<(?:a|img|source|video)\b[^>]*?\s(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)')/gi)) {
      yield { line: i + 1, target: m[1] ?? m[2] };
    }
  }
}

const isExternal = (target) => /^[a-z][a-z0-9+.-]*:/i.test(target) || target.startsWith('//');

// Why the link from `file` to `target` is broken, or null if it is fine.
function problem(file, text, target) {
  const hash = target.indexOf('#');
  const fragment = hash >= 0 ? target.slice(hash + 1) : '';
  let path;
  try {
    path = decodeURIComponent((hash >= 0 ? target.slice(0, hash) : target).replace(/\?.*$/, ''));
  } catch {
    return 'not a valid URL';
  }
  let resolved = file;
  if (path !== '') {
    resolved = posix.normalize(path.startsWith('/') ? path.slice(1) : posix.join(posix.dirname(file), path));
    resolved = resolved.replace(/\/+$/, '');
    if (resolved === '.') resolved = '';
    if (resolved === '..' || resolved.startsWith('../')) return 'points outside the repository';
    if (!files.has(resolved) && !dirs.has(resolved)) return `no such file or directory: ${resolved || '/'}`;
  }
  if (fragment === '' || !/\.md$/i.test(resolved)) return null;
  let id;
  try {
    id = decodeURIComponent(fragment);
  } catch {
    return 'the #fragment is not a valid URL';
  }
  const ids = path === '' ? idsOf(file, text) : idsOf(resolved);
  // GitHub also answers to the ids with its user-content- prefix.
  const bare = id.replace(/^user-content-/, '');
  return ids.has(bare) || ids.has(bare.toLowerCase()) ? null : `no heading #${id} in ${resolved}`;
}

const markdown =
  process.argv.length > 2
    ? process.argv.slice(2).map((file) => posix.relative(root, posix.resolve(file.replaceAll('\\', '/'))))
    : [...files].filter((file) => /\.md$/i.test(file)).sort();
const annotate = process.env.GITHUB_ACTIONS === 'true';
let checked = 0;
let broken = 0;
for (const file of markdown) {
  let text;
  try {
    text = read(file);
  } catch (error) {
    console.log(`${file}: cannot read it: ${error.message}`);
    broken++;
    continue;
  }
  for (const { line, target: raw } of links(text)) {
    const target = raw.startsWith('<') && raw.endsWith('>') ? raw.slice(1, -1) : raw;
    if (isExternal(target)) continue;
    checked++;
    const why = problem(file, text, target);
    if (why === null) continue;
    broken++;
    const message = `broken link ${target}: ${why}`;
    console.log(annotate ? `::error file=${file},line=${line}::${message}` : `${file}:${line}: ${message}`);
  }
}

console.log(`${broken === 0 ? 'ok' : `${broken} broken`}: ${checked} relative links in ${markdown.length} Markdown files`);
process.exit(broken === 0 ? 0 : 1);
